package process

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// AuthCommand represents an authentication command type
type AuthCommand int

const (
	ClaudeLogin AuthCommand = iota
	CodexLogin
	GeminiLogin
	QwenLogin
)

// Manager manages the CLIProxyAPI backend process
type Manager struct {
	mu           sync.RWMutex
	cmd          *exec.Cmd
	isRunning    bool
	logBuffer    *RingBuffer
	binaryPath   string
	configPath   string
	onLogUpdate  func([]string)
	cancelOutput chan struct{}
}

// RingBuffer implements a fixed-size circular buffer for log lines
type RingBuffer struct {
	mu      sync.Mutex
	storage []string
	head    int
	tail    int
	count   int
	cap     int
}

// NewRingBuffer creates a new ring buffer with the specified capacity
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity < 1 {
		capacity = 1
	}
	return &RingBuffer{
		storage: make([]string, capacity),
		cap:     capacity,
	}
}

// Append adds an element to the ring buffer
func (rb *RingBuffer) Append(line string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.storage[rb.tail] = line

	if rb.count == rb.cap {
		rb.head = (rb.head + 1) % rb.cap
	} else {
		rb.count++
	}

	rb.tail = (rb.tail + 1) % rb.cap
}

// Elements returns all elements in the ring buffer in order
func (rb *RingBuffer) Elements() []string {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count == 0 {
		return []string{}
	}

	result := make([]string, 0, rb.count)
	for i := 0; i < rb.count; i++ {
		idx := (rb.head + i) % rb.cap
		result = append(result, rb.storage[idx])
	}

	return result
}

// NewManager creates a new process manager
func NewManager(binaryPath, configPath string) *Manager {
	return &Manager{
		logBuffer:  NewRingBuffer(1000),
		binaryPath: binaryPath,
		configPath: configPath,
	}
}

// IsRunning returns true if the server is running
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}

// HealthCheck checks if CLIProxyAPI is actually listening on port 8318
func (m *Manager) HealthCheck() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:8318", 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Start starts the CLIProxyAPI server
func (m *Manager) Start() error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	// Kill any orphaned processes
	m.killOrphanedProcesses()

	// Verify binary exists
	if _, err := os.Stat(m.binaryPath); err != nil {
		return fmt.Errorf("binary not found at %s: %w", m.binaryPath, err)
	}

	// Verify config exists
	if _, err := os.Stat(m.configPath); err != nil {
		return fmt.Errorf("config not found at %s: %w", m.configPath, err)
	}

	// Create command
	cmd := exec.Command(m.binaryPath, "--config", m.configPath)

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	m.mu.Lock()
	m.cmd = cmd
	m.isRunning = true
	m.cancelOutput = make(chan struct{})
	m.mu.Unlock()

	m.addLog("✓ Server started on port 8318")

	// Start output readers
	go m.readOutput(stdout, "")
	go m.readOutput(stderr, "⚠️ ")

	// Wait for process in background
	go func() {
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}

		m.mu.Lock()
		m.isRunning = false
		m.cmd = nil
		if m.cancelOutput != nil {
			close(m.cancelOutput)
			m.cancelOutput = nil
		}
		m.mu.Unlock()

		m.addLog(fmt.Sprintf("Server stopped with code: %d", exitCode))
	}()

	// Wait a bit to ensure it started successfully
	time.Sleep(1 * time.Second)

	return nil
}

// Stop stops the CLIProxyAPI server
func (m *Manager) Stop() error {
	m.mu.Lock()
	cmd := m.cmd
	isRunning := m.isRunning
	m.mu.Unlock()

	if !isRunning || cmd == nil {
		m.mu.Lock()
		m.isRunning = false
		m.mu.Unlock()
		return nil
	}

	pid := cmd.Process.Pid
	m.addLog(fmt.Sprintf("Stopping server (PID: %d)...", pid))

	// Try graceful termination (SIGTERM)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("[Process] Failed to send SIGTERM: %v", err)
	}

	// Wait up to 2 seconds for graceful termination
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// Graceful shutdown succeeded
		m.addLog("✓ Server stopped gracefully")
	case <-time.After(2 * time.Second):
		// Force kill
		m.addLog("⚠️ Server didn't stop gracefully, force killing...")
		if err := cmd.Process.Kill(); err != nil {
			log.Printf("[Process] Failed to kill process: %v", err)
		}
		<-done // Wait for process to actually exit
	}

	m.mu.Lock()
	m.cmd = nil
	m.isRunning = false
	if m.cancelOutput != nil {
		close(m.cancelOutput)
		m.cancelOutput = nil
	}
	m.mu.Unlock()

	return nil
}

// RunAuthCommand executes an authentication command
func (m *Manager) RunAuthCommand(command AuthCommand, email string) (bool, string, error) {
	// Verify binary exists
	if _, err := os.Stat(m.binaryPath); err != nil {
		return false, "", fmt.Errorf("binary not found at %s", m.binaryPath)
	}

	args := []string{"--config", m.configPath}

	var cmdType string
	switch command {
	case ClaudeLogin:
		args = append(args, "-claude-login")
		cmdType = "Claude"
	case CodexLogin:
		args = append(args, "-codex-login")
		cmdType = "Codex"
	case GeminiLogin:
		args = append(args, "-login")
		cmdType = "Gemini"
	case QwenLogin:
		args = append(args, "-qwen-login")
		cmdType = "Qwen"
	default:
		return false, "", fmt.Errorf("unknown auth command")
	}

	cmd := exec.Command(m.binaryPath, args...)

	// Create pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return false, "", err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, "", err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return false, "", err
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return false, "", fmt.Errorf("failed to start auth process: %w", err)
	}

	log.Printf("[Auth] Starting %s authentication process (PID: %d)", cmdType, cmd.Process.Pid)
	m.addLog(fmt.Sprintf("✓ Authentication process started (PID: %d) - browser should open shortly", cmd.Process.Pid))

	// Handle Gemini auto-newline
	if command == GeminiLogin {
		go func() {
			time.Sleep(3 * time.Second)
			stdin.Write([]byte("\n"))
			log.Printf("[Auth] Sent newline to accept default project")
		}()
	}

	// Handle Qwen auto-email
	if command == QwenLogin && email != "" {
		go func() {
			time.Sleep(10 * time.Second)
			stdin.Write([]byte(email + "\n"))
			log.Printf("[Auth] Sent Qwen email: %s", email)
		}()
	}

	// Read output
	outputBuf := &strings.Builder{}
	errorBuf := &strings.Builder{}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			outputBuf.WriteString(line + "\n")
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			errorBuf.WriteString(line + "\n")
		}
	}()

	// Wait briefly to check if process crashes immediately
	time.Sleep(1 * time.Second)

	// Check if process is still running
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		// Process finished
		output := outputBuf.String()
		errorOutput := errorBuf.String()

		if err == nil || strings.Contains(output, "Opening browser") || strings.Contains(output, "Attempting to open URL") {
			return true, "🌐 Browser opened for authentication.\n\nPlease complete the login in your browser.\n\nThe app will automatically detect when you're authenticated.", nil
		}

		message := errorOutput
		if message == "" {
			message = output
		}
		if message == "" {
			message = "Authentication process failed unexpectedly"
		}

		return false, message, err

	case <-time.After(500 * time.Millisecond):
		// Process is still running after 1.5s - likely succeeded
		return true, "🌐 Browser opened for authentication.\n\nPlease complete the login in your browser.\n\nThe app will automatically detect when you're authenticated.", nil
	}
}

// GetLogs returns all log lines
func (m *Manager) GetLogs() []string {
	return m.logBuffer.Elements()
}

// addLog adds a log line to the buffer
func (m *Manager) addLog(message string) {
	timestamp := time.Now().Format("15:04:05")
	logLine := fmt.Sprintf("[%s] %s", timestamp, message)
	m.logBuffer.Append(logLine)

	if m.onLogUpdate != nil {
		m.onLogUpdate(m.logBuffer.Elements())
	}
}

// readOutput reads from an output pipe and adds to logs
func (m *Manager) readOutput(reader io.Reader, prefix string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		select {
		case <-m.cancelOutput:
			return
		default:
			line := scanner.Text()
			if line != "" {
				m.addLog(prefix + line)
			}
		}
	}
}

// killOrphanedProcesses kills any orphaned cli-proxy-api processes
func (m *Manager) killOrphanedProcesses() {
	// Use pgrep to find processes
	cmd := exec.Command("pgrep", "-f", "cli-proxy-api")
	output, err := cmd.Output()
	if err != nil {
		// Exit code 1 means no processes found - this is fine
		return
	}

	pids := strings.TrimSpace(string(output))
	if pids == "" {
		return
	}

	m.addLog(fmt.Sprintf("⚠️ Found orphaned server process(es): %s", pids))

	// Kill them
	killCmd := exec.Command("pkill", "-9", "-f", "cli-proxy-api")
	if err := killCmd.Run(); err != nil {
		log.Printf("[Process] Failed to kill orphaned processes: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	m.addLog("✓ Cleaned up orphaned processes")
}

// GetBinaryPath returns the path to the bundled CLI proxy binary
func GetBinaryPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	execDir := filepath.Dir(execPath)

	// Check if cli-proxy-api is in the same directory as the executable
	binaryPath := filepath.Join(execDir, "cli-proxy-api")
	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath, nil
	}

	return "", fmt.Errorf("cli-proxy-api binary not found")
}

// GetConfigPath returns the path to the config file, auto-creating it if needed
func GetConfigPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	execDir := filepath.Dir(execPath)

	// Check if config.yaml is in the same directory as the executable
	configPath := filepath.Join(execDir, "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return configPath, nil
	}

	// Check for config.default.yaml template
	defaultConfigPath := filepath.Join(execDir, "config.default.yaml")
	if _, err := os.Stat(defaultConfigPath); err == nil {
		// Copy default config to config.yaml
		log.Printf("[Config] Creating config.yaml from default template")
		if err := copyFile(defaultConfigPath, configPath); err != nil {
			return "", fmt.Errorf("failed to create config from template: %w", err)
		}
		log.Printf("[Config] Created config.yaml at %s", configPath)
		return configPath, nil
	}

	// Last resort: create minimal config
	log.Printf("[Config] No template found, creating minimal config.yaml")
	if err := createMinimalConfig(configPath); err != nil {
		return "", fmt.Errorf("failed to create minimal config: %w", err)
	}
	log.Printf("[Config] Created minimal config.yaml at %s", configPath)
	return configPath, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// createMinimalConfig creates a minimal config.yaml
func createMinimalConfig(path string) error {
	content := `# CLIProxyAPI Configuration (auto-generated)
# Backend port for CLIProxyAPI (DO NOT CHANGE)
port: 8318

# Directory where authentication tokens are stored
auth-dir: ~/.cli-proxy-api

# Remote management
remote-management:
  allow-remote: false
  secret-key: ""
  disable-control-panel: false

# Client API keys
api-keys:
  - dummy-not-used

# Settings
debug: false
logging-to-file: false
usage-statistics-enabled: false
proxy-url: ''
request-retry: 3

quota-exceeded:
  switch-project: true
  switch-preview-model: true

ws-auth: false
`
	return os.WriteFile(path, []byte(content), 0644)
}
