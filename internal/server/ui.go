package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/automazeio/vibeproxy/internal/auth"
	"github.com/automazeio/vibeproxy/internal/process"
)

//go:embed static/*
var staticFiles embed.FS

// UIServer serves the web UI
type UIServer struct {
	port           int
	authManager    *auth.Manager
	processManager *process.Manager
	mux            *http.ServeMux
}

// NewUIServer creates a new UI server
func NewUIServer(port int, authMgr *auth.Manager, procMgr *process.Manager) *UIServer {
	s := &UIServer{
		port:           port,
		authManager:    authMgr,
		processManager: procMgr,
		mux:            http.NewServeMux(),
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all HTTP routes
func (s *UIServer) setupRoutes() {
	// API routes
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/auth/connect", s.handleConnect)
	s.mux.HandleFunc("/api/auth/disconnect", s.handleDisconnect)
	s.mux.HandleFunc("/api/server/start", s.handleServerStart)
	s.mux.HandleFunc("/api/server/stop", s.handleServerStop)
	s.mux.HandleFunc("/api/autostart/enable", s.handleAutostartEnable)
	s.mux.HandleFunc("/api/autostart/disable", s.handleAutostartDisable)
	s.mux.HandleFunc("/api/autostart/status", s.handleAutostartStatus)

	// Static files
	s.mux.Handle("/", http.FileServer(http.FS(staticFiles)))
}

// Start starts the UI server
func (s *UIServer) Start() error {
	log.Printf("[UIServer] Starting on port %d", s.port)
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", s.port), s.mux); err != nil {
			log.Printf("[UIServer] Server error: %v", err)
		}
	}()
	return nil
}

// handleStatus returns the current status of all services
func (s *UIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Refresh auth status
	if err := s.authManager.CheckAuthStatus(); err != nil {
		log.Printf("[UIServer] Error checking auth status: %v", err)
	}

	status := map[string]interface{}{
		"services": s.authManager.GetStatus(),
		"server": map[string]interface{}{
			"running": s.processManager.IsRunning() && s.processManager.HealthCheck(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleConnect handles authentication requests
func (s *UIServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Service string `json:"service"`
		Email   string `json:"email,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var cmd process.AuthCommand
	switch strings.ToLower(req.Service) {
	case "claude":
		cmd = process.ClaudeLogin
	case "codex":
		cmd = process.CodexLogin
	case "gemini":
		cmd = process.GeminiLogin
	case "qwen":
		cmd = process.QwenLogin
		if req.Email == "" {
			http.Error(w, "Email required for Qwen", http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "Unknown service", http.StatusBadRequest)
		return
	}

	success, message, err := s.processManager.RunAuthCommand(cmd, req.Email)

	response := map[string]interface{}{
		"success": success,
		"message": message,
	}

	if err != nil {
		response["error"] = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDisconnect handles disconnection requests
func (s *UIServer) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Service string `json:"service"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		http.Error(w, "Failed to get home directory", http.StatusInternalServerError)
		return
	}

	authDir := filepath.Join(homeDir, ".cli-proxy-api")
	serviceType := strings.ToLower(req.Service)

	// Find and delete the auth file
	files, err := os.ReadDir(authDir)
	if err != nil {
		http.Error(w, "Failed to read auth directory", http.StatusInternalServerError)
		return
	}

	found := false
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(authDir, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var authData map[string]interface{}
		if err := json.Unmarshal(data, &authData); err != nil {
			continue
		}

		fileType, ok := authData["type"].(string)
		if !ok || strings.ToLower(fileType) != serviceType {
			continue
		}

		// Found the file - delete it
		if err := os.Remove(filePath); err != nil {
			http.Error(w, "Failed to delete auth file", http.StatusInternalServerError)
			return
		}

		found = true
		break
	}

	if !found {
		http.Error(w, "Auth file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("%s disconnected successfully", req.Service),
	})
}

// handleServerStart starts the backend server
func (s *UIServer) handleServerStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.processManager.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// handleServerStop stops the backend server
func (s *UIServer) handleServerStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.processManager.Stop(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// handleAutostartEnable enables autostart
func (s *UIServer) handleAutostartEnable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := enableAutostart(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// handleAutostartDisable disables autostart
func (s *UIServer) handleAutostartDisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := disableAutostart(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// handleAutostartStatus returns autostart status
func (s *UIServer) handleAutostartStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	enabled := isAutostartEnabled()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled": enabled,
	})
}

// enableAutostart enables the application to start on boot
func enableAutostart() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("autostart only supported on Linux")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	autostartDir := filepath.Join(homeDir, ".config", "autostart")
	if err := os.MkdirAll(autostartDir, 0755); err != nil {
		return err
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	desktopEntry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=VibeProxy
Exec=%s
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true
`, execPath)

	desktopFile := filepath.Join(autostartDir, "vibeproxy.desktop")
	return os.WriteFile(desktopFile, []byte(desktopEntry), 0644)
}

// disableAutostart disables autostart
func disableAutostart() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("autostart only supported on Linux")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	desktopFile := filepath.Join(homeDir, ".config", "autostart", "vibeproxy.desktop")
	if err := os.Remove(desktopFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// isAutostartEnabled checks if autostart is enabled
func isAutostartEnabled() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	desktopFile := filepath.Join(homeDir, ".config", "autostart", "vibeproxy.desktop")
	_, err = os.Stat(desktopFile)
	return err == nil
}

// OpenBrowser opens the default browser to the UI
func OpenBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
