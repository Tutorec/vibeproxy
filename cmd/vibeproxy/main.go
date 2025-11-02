package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/automazeio/vibeproxy/internal/auth"
	"github.com/automazeio/vibeproxy/internal/process"
	"github.com/automazeio/vibeproxy/internal/proxy"
	"github.com/automazeio/vibeproxy/internal/server"
)

const (
	thinkingProxyPort = 8317 // Client-facing port with thinking transformation
	cliProxyAPIPort   = 8318 // Backend CLIProxyAPI port
	uiServerPort      = 8319 // Web UI port
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[VibeProxy] Starting...")

	// Get binary and config paths
	binaryPath, err := process.GetBinaryPath()
	if err != nil {
		log.Fatalf("[VibeProxy] Failed to find cli-proxy-api binary: %v", err)
	}
	log.Printf("[VibeProxy] Using binary: %s", binaryPath)

	configPath, err := process.GetConfigPath()
	if err != nil {
		log.Fatalf("[VibeProxy] Failed to find config.yaml: %v", err)
	}
	log.Printf("[VibeProxy] Using config: %s", configPath)

	// Create auth manager
	authManager := auth.NewManager()
	if err := authManager.CheckAuthStatus(); err != nil {
		log.Printf("[VibeProxy] Warning: Failed to check auth status: %v", err)
	}

	// Create process manager for CLIProxyAPI
	processManager := process.NewManager(binaryPath, configPath)

	// Create thinking proxy (8317 → 8318)
	thinkingProxy := proxy.NewThinkingProxy(thinkingProxyPort, cliProxyAPIPort)

	// Create web UI server
	uiServer := server.NewUIServer(uiServerPort, authManager, processManager)

	// Create file watcher for auth directory
	watcher, err := auth.NewWatcher(authManager, func() {
		log.Println("[VibeProxy] Auth status changed")
	})
	if err != nil {
		log.Printf("[VibeProxy] Warning: Failed to create file watcher: %v", err)
	} else {
		defer watcher.Close()
	}

	// Start thinking proxy first
	if err := thinkingProxy.Start(); err != nil {
		log.Fatalf("[VibeProxy] Failed to start thinking proxy: %v", err)
	}
	log.Printf("[VibeProxy] ThinkingProxy started on port %d", thinkingProxyPort)

	// Wait for thinking proxy to be ready
	time.Sleep(100 * time.Millisecond)

	// Start CLIProxyAPI backend
	if err := processManager.Start(); err != nil {
		log.Fatalf("[VibeProxy] Failed to start CLIProxyAPI: %v", err)
	}
	log.Printf("[VibeProxy] CLIProxyAPI started on port %d", cliProxyAPIPort)

	// Wait for CLIProxyAPI to be ready (actually listening on port 8318)
	log.Println("[VibeProxy] Waiting for CLIProxyAPI to be ready...")
	ready := false
	for i := 0; i < 30; i++ {
		if processManager.HealthCheck() {
			ready = true
			log.Println("[VibeProxy] CLIProxyAPI is ready and accepting connections")
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if !ready {
		log.Fatalf("[VibeProxy] CLIProxyAPI failed to start (port 8318 not listening after 15 seconds)")
	}

	// Start web UI server
	if err := uiServer.Start(); err != nil {
		log.Fatalf("[VibeProxy] Failed to start UI server: %v", err)
	}
	log.Printf("[VibeProxy] Web UI started on port %d", uiServerPort)

	// Open browser to UI
	uiURL := fmt.Sprintf("http://localhost:%d/static/", uiServerPort)
	log.Printf("[VibeProxy] Opening browser to %s", uiURL)
	if err := server.OpenBrowser(uiURL); err != nil {
		log.Printf("[VibeProxy] Failed to open browser: %v", err)
		log.Printf("[VibeProxy] Please open manually: %s", uiURL)
	}

	log.Println("[VibeProxy] All services started successfully!")
	log.Printf("[VibeProxy] Client port: %d (with thinking transformation)", thinkingProxyPort)
	log.Printf("[VibeProxy] Backend port: %d (CLIProxyAPI)", cliProxyAPIPort)
	log.Printf("[VibeProxy] Web UI: %s", uiURL)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("[VibeProxy] Shutting down...")

	// Stop all services
	if err := thinkingProxy.Stop(); err != nil {
		log.Printf("[VibeProxy] Error stopping thinking proxy: %v", err)
	}

	if err := processManager.Stop(); err != nil {
		log.Printf("[VibeProxy] Error stopping process manager: %v", err)
	}

	log.Println("[VibeProxy] Shutdown complete")
}
