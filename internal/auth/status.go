package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AuthStatus represents the authentication status for a single service
type AuthStatus struct {
	IsAuthenticated bool      `json:"isAuthenticated"`
	Email           string    `json:"email,omitempty"`
	Type            string    `json:"type"`
	Expired         time.Time `json:"expired,omitempty"`
}

// IsExpired checks if the authentication has expired
func (a *AuthStatus) IsExpired() bool {
	if a.Expired.IsZero() {
		return false
	}
	return a.Expired.Before(time.Now())
}

// StatusText returns a human-readable status text
func (a *AuthStatus) StatusText() string {
	if !a.IsAuthenticated {
		return "Not Connected"
	}
	if a.IsExpired() {
		return "Expired - Reconnect Required"
	}
	if a.Email != "" {
		return fmt.Sprintf("Connected as %s", a.Email)
	}
	return "Connected"
}

// Manager manages authentication status for all services
type Manager struct {
	Claude AuthStatus
	Codex  AuthStatus
	Gemini AuthStatus
	Qwen   AuthStatus
}

// NewManager creates a new AuthManager
func NewManager() *Manager {
	return &Manager{
		Claude: AuthStatus{Type: "claude"},
		Codex:  AuthStatus{Type: "codex"},
		Gemini: AuthStatus{Type: "gemini"},
		Qwen:   AuthStatus{Type: "qwen"},
	}
}

// authFileData represents the structure of auth JSON files
type authFileData struct {
	Type         string `json:"type"`
	Email        string `json:"email"`
	Expired      string `json:"expired"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

// CheckAuthStatus scans the ~/.cli-proxy-api directory and updates auth status
func (m *Manager) CheckAuthStatus() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	authDir := filepath.Join(homeDir, ".cli-proxy-api")

	// Reset all statuses first
	foundClaude := false
	foundCodex := false
	foundGemini := false
	foundQwen := false

	// Check if directory exists
	if _, err := os.Stat(authDir); os.IsNotExist(err) {
		// Directory doesn't exist yet - all services unauthenticated
		m.resetAll()
		return nil
	}

	// Read all files in the directory
	files, err := os.ReadDir(authDir)
	if err != nil {
		m.resetAll()
		return fmt.Errorf("failed to read auth directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(authDir, file.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var authData authFileData
		if err := json.Unmarshal(data, &authData); err != nil {
			continue
		}

		// Parse expiration date
		var expiredTime time.Time
		if authData.Expired != "" {
			// Try ISO8601 format with fractional seconds
			expiredTime, _ = time.Parse(time.RFC3339Nano, authData.Expired)
		}

		status := AuthStatus{
			IsAuthenticated: true,
			Email:           authData.Email,
			Type:            authData.Type,
			Expired:         expiredTime,
		}

		// Update the appropriate service status
		switch strings.ToLower(authData.Type) {
		case "claude":
			foundClaude = true
			m.Claude = status
		case "codex":
			foundCodex = true
			m.Codex = status
		case "gemini":
			foundGemini = true
			m.Gemini = status
		case "qwen":
			foundQwen = true
			m.Qwen = status
		}
	}

	// Reset services without auth files
	if !foundClaude {
		m.Claude = AuthStatus{Type: "claude"}
	}
	if !foundCodex {
		m.Codex = AuthStatus{Type: "codex"}
	}
	if !foundGemini {
		m.Gemini = AuthStatus{Type: "gemini"}
	}
	if !foundQwen {
		m.Qwen = AuthStatus{Type: "qwen"}
	}

	return nil
}

// resetAll resets all service statuses to unauthenticated
func (m *Manager) resetAll() {
	m.Claude = AuthStatus{Type: "claude"}
	m.Codex = AuthStatus{Type: "codex"}
	m.Gemini = AuthStatus{Type: "gemini"}
	m.Qwen = AuthStatus{Type: "qwen"}
}

// GetStatus returns a map of all service statuses for JSON serialization
func (m *Manager) GetStatus() map[string]AuthStatus {
	return map[string]AuthStatus{
		"claude": m.Claude,
		"codex":  m.Codex,
		"gemini": m.Gemini,
		"qwen":   m.Qwen,
	}
}
