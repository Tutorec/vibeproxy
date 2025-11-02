package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// ThinkingProxy is a lightweight HTTP proxy that intercepts requests to add
// extended thinking parameters for Claude models based on model name suffixes.
//
// Model name pattern:
// - `*-thinking-NUMBER` → Custom token budget (e.g., claude-sonnet-4-5-20250929-thinking-5000)
//
// The proxy strips the suffix and adds the `thinking` parameter to the request body
// before forwarding to CLIProxyAPI.
type ThinkingProxy struct {
	mu         sync.RWMutex
	listener   net.Listener
	proxyPort  int
	targetPort int
	targetHost string
	isRunning  bool
	done       chan struct{}
}

// NewThinkingProxy creates a new thinking proxy
func NewThinkingProxy(proxyPort, targetPort int) *ThinkingProxy {
	return &ThinkingProxy{
		proxyPort:  proxyPort,
		targetPort: targetPort,
		targetHost: "127.0.0.1",
		done:       make(chan struct{}),
	}
}

// Start starts the thinking proxy server
func (tp *ThinkingProxy) Start() error {
	tp.mu.Lock()
	if tp.isRunning {
		tp.mu.Unlock()
		log.Printf("[ThinkingProxy] Already running")
		return nil
	}
	tp.mu.Unlock()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", tp.proxyPort))
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	tp.mu.Lock()
	tp.listener = listener
	tp.isRunning = true
	tp.mu.Unlock()

	log.Printf("[ThinkingProxy] Listening on port %d", tp.proxyPort)

	go tp.acceptConnections()

	return nil
}

// Stop stops the thinking proxy server
func (tp *ThinkingProxy) Stop() error {
	tp.mu.Lock()
	if !tp.isRunning {
		tp.mu.Unlock()
		return nil
	}
	tp.mu.Unlock()

	close(tp.done)

	tp.mu.Lock()
	if tp.listener != nil {
		tp.listener.Close()
	}
	tp.isRunning = false
	tp.mu.Unlock()

	log.Printf("[ThinkingProxy] Stopped")
	return nil
}

// IsRunning returns true if the proxy is running
func (tp *ThinkingProxy) IsRunning() bool {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	return tp.isRunning
}

// acceptConnections accepts incoming connections
func (tp *ThinkingProxy) acceptConnections() {
	for {
		conn, err := tp.listener.Accept()
		if err != nil {
			select {
			case <-tp.done:
				return
			default:
				log.Printf("[ThinkingProxy] Accept error: %v", err)
				continue
			}
		}

		go tp.handleConnection(conn)
	}
}

// handleConnection handles a single client connection
func (tp *ThinkingProxy) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	// Read the HTTP request
	reader := bufio.NewReader(clientConn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		tp.sendError(clientConn, http.StatusBadRequest, "Invalid request")
		return
	}

	// Read request body
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			tp.sendError(clientConn, http.StatusBadRequest, "Failed to read body")
			return
		}
	}

	// Process thinking parameter for POST requests with JSON body
	modifiedBody := bodyBytes
	transformationApplied := false

	if req.Method == "POST" && len(bodyBytes) > 0 {
		if modified, applied := tp.processThinkingParameter(bodyBytes); modified != nil {
			modifiedBody = modified
			transformationApplied = applied
		}
	}

	// Forward request to CLIProxyAPI
	tp.forwardRequest(req, modifiedBody, clientConn, transformationApplied)
}

// processThinkingParameter processes the JSON body to add thinking parameter
// Returns (modifiedJSON, needsTransformation)
func (tp *ThinkingProxy) processThinkingParameter(bodyBytes []byte) ([]byte, bool) {
	var jsonBody map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &jsonBody); err != nil {
		return bodyBytes, false
	}

	model, ok := jsonBody["model"].(string)
	if !ok || !strings.HasPrefix(model, "claude-") {
		return bodyBytes, false
	}

	// Check for thinking suffix pattern: -thinking-NUMBER
	thinkingPrefix := "-thinking-"
	idx := strings.LastIndex(model, thinkingPrefix)
	if idx == -1 {
		return bodyBytes, false
	}

	// Extract the budget number after "-thinking-"
	budgetStr := model[idx+len(thinkingPrefix):]
	cleanModel := model[:idx]

	// Strip the thinking suffix from model name
	jsonBody["model"] = cleanModel

	// Only add thinking parameter if it's a valid integer
	budget, err := strconv.Atoi(budgetStr)
	if err != nil || budget <= 0 {
		log.Printf("[ThinkingProxy] Stripped invalid thinking suffix from '%s' → '%s' (no thinking)", model, cleanModel)
		modified, _ := json.Marshal(jsonBody)
		return modified, true
	}

	// Apply hard cap
	const hardCap = 32000
	effectiveBudget := budget
	if effectiveBudget >= hardCap {
		effectiveBudget = hardCap - 1
		log.Printf("[ThinkingProxy] Adjusted thinking budget from %d to %d to stay within limits", budget, effectiveBudget)
	}

	// Add thinking parameter
	jsonBody["thinking"] = map[string]interface{}{
		"type":          "enabled",
		"budget_tokens": effectiveBudget,
	}

	// Ensure max token limits are greater than the thinking budget
	tokenHeadroom := 1024
	if effectiveBudget/10 > tokenHeadroom {
		tokenHeadroom = effectiveBudget / 10
	}

	desiredMaxTokens := effectiveBudget + tokenHeadroom
	requiredMaxTokens := desiredMaxTokens
	if requiredMaxTokens > hardCap {
		requiredMaxTokens = hardCap
	}
	if requiredMaxTokens <= effectiveBudget {
		requiredMaxTokens = effectiveBudget + 1
		if requiredMaxTokens > hardCap {
			requiredMaxTokens = hardCap
		}
	}

	// Check if max_output_tokens field exists
	_, hasMaxOutputTokens := jsonBody["max_output_tokens"]
	adjusted := false

	if maxTokens, ok := jsonBody["max_tokens"].(float64); ok {
		if int(maxTokens) <= effectiveBudget {
			jsonBody["max_tokens"] = requiredMaxTokens
		}
		adjusted = true
	}

	if maxOutputTokens, ok := jsonBody["max_output_tokens"].(float64); ok {
		if int(maxOutputTokens) <= effectiveBudget {
			jsonBody["max_output_tokens"] = requiredMaxTokens
		}
		adjusted = true
	}

	if !adjusted {
		if hasMaxOutputTokens {
			jsonBody["max_output_tokens"] = requiredMaxTokens
		} else {
			jsonBody["max_tokens"] = requiredMaxTokens
		}
	}

	log.Printf("[ThinkingProxy] Transformed model '%s' → '%s' with thinking budget %d", model, cleanModel, effectiveBudget)

	modified, err := json.Marshal(jsonBody)
	if err != nil {
		return bodyBytes, false
	}

	return modified, true
}

// forwardRequest forwards the request to CLIProxyAPI
func (tp *ThinkingProxy) forwardRequest(req *http.Request, body []byte, clientConn net.Conn, forceConnectionClose bool) {
	// Connect to CLIProxyAPI
	targetConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", tp.targetHost, tp.targetPort))
	if err != nil {
		log.Printf("[ThinkingProxy] Failed to connect to target: %v", err)
		tp.sendError(clientConn, http.StatusBadGateway, "Bad Gateway")
		return
	}
	defer targetConn.Close()

	// Build forwarded request
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%s %s %s\r\n", req.Method, req.URL.RequestURI(), req.Proto))

	// Copy headers except excluded ones
	excludedHeaders := map[string]bool{
		"content-length":    true,
		"host":              true,
		"transfer-encoding": true,
	}

	for name, values := range req.Header {
		if excludedHeaders[strings.ToLower(name)] {
			continue
		}
		for _, value := range values {
			buf.WriteString(fmt.Sprintf("%s: %s\r\n", name, value))
		}
	}

	// Add required headers
	buf.WriteString(fmt.Sprintf("Host: %s:%d\r\n", tp.targetHost, tp.targetPort))
	buf.WriteString("Connection: close\r\n") // Always close connections
	buf.WriteString(fmt.Sprintf("Content-Length: %d\r\n", len(body)))
	buf.WriteString("\r\n")
	buf.Write(body)

	// Send to CLIProxyAPI
	if _, err := targetConn.Write(buf.Bytes()); err != nil {
		log.Printf("[ThinkingProxy] Send error: %v", err)
		return
	}

	// Stream response back to client
	tp.streamResponse(targetConn, clientConn)
}

// streamResponse streams the response from target to client
func (tp *ThinkingProxy) streamResponse(targetConn, clientConn net.Conn) {
	buf := make([]byte, 65536)
	for {
		n, err := targetConn.Read(buf)
		if n > 0 {
			if _, writeErr := clientConn.Write(buf[:n]); writeErr != nil {
				log.Printf("[ThinkingProxy] Write error: %v", writeErr)
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("[ThinkingProxy] Read error: %v", err)
			}
			return
		}
	}
}

// sendError sends an HTTP error response
func (tp *ThinkingProxy) sendError(conn net.Conn, statusCode int, message string) {
	body := message
	response := fmt.Sprintf("HTTP/1.1 %d %s\r\n"+
		"Content-Type: text/plain\r\n"+
		"Content-Length: %d\r\n"+
		"Connection: close\r\n"+
		"\r\n"+
		"%s", statusCode, message, len(body), body)

	conn.Write([]byte(response))
	conn.Close()
}
