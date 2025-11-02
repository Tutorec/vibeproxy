package auth

import (
	"log"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors the auth directory for changes
type Watcher struct {
	watcher  *fsnotify.Watcher
	manager  *Manager
	onChange func()
	done     chan struct{}
}

// NewWatcher creates a new file system watcher for the auth directory
func NewWatcher(manager *Manager, onChange func()) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		watcher.Close()
		return nil, err
	}

	authDir := filepath.Join(homeDir, ".cli-proxy-api")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(authDir, 0755); err != nil {
		watcher.Close()
		return nil, err
	}

	// Add directory to watcher
	if err := watcher.Add(authDir); err != nil {
		watcher.Close()
		return nil, err
	}

	w := &Watcher{
		watcher:  watcher,
		manager:  manager,
		onChange: onChange,
		done:     make(chan struct{}),
	}

	go w.watch()

	log.Printf("[FileWatcher] Monitoring auth directory: %s", authDir)

	return w, nil
}

// watch runs the file system monitoring loop
func (w *Watcher) watch() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Trigger on any write, create, remove, or rename event
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				log.Printf("[FileWatcher] Auth directory changed: %s", event.Name)

				// Refresh auth status
				if err := w.manager.CheckAuthStatus(); err != nil {
					log.Printf("[FileWatcher] Error checking auth status: %v", err)
				}

				// Notify callback
				if w.onChange != nil {
					w.onChange()
				}
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[FileWatcher] Error: %v", err)

		case <-w.done:
			return
		}
	}
}

// Close stops the watcher
func (w *Watcher) Close() error {
	close(w.done)
	return w.watcher.Close()
}
