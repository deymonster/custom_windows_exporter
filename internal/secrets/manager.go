package secrets

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type Manager struct {
	mu               sync.RWMutex
	handshakeKey     string
	passwordHash     string
	handshakeFile    string
	passwordFile     string
	passwordHashFile string

	watchMu  sync.Mutex
	watchers map[string]*fsnotify.Watcher
}

func NewManager() *Manager {
	return &Manager{watchers: make(map[string]*fsnotify.Watcher)}
}

func (m *Manager) Reload() error {
	handshake, handshakeFile, err := readSecret("NCM_HANDSHAKE_KEY", "NCM_HANDSHAKE_KEY_FILE")
	if err != nil {
		return fmt.Errorf("reload handshake key: %w", err)
	}

	m.mu.Lock()
	m.handshakeKey = handshake
	m.handshakeFile = handshakeFile
	m.mu.Unlock()

	passwordHash, passwordFile, passwordHashFile, err := readPassword()
	if err != nil {
		return fmt.Errorf("reload api password: %w", err)
	}

	m.mu.Lock()
	m.passwordHash = strings.ToLower(passwordHash)
	m.passwordFile = passwordFile
	m.passwordHashFile = passwordHashFile
	m.mu.Unlock()

	return nil
}

func (m *Manager) HandshakeKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.handshakeKey
}

func (m *Manager) ValidatePassword(candidate string) bool {
	m.mu.RLock()
	hash := m.passwordHash
	m.mu.RUnlock()
	if hash == "" {
		return false
	}

	candidateHash := sha256.Sum256([]byte(candidate))
	return hex.EncodeToString(candidateHash[:]) == hash
}

func (m *Manager) WatchFiles(ctxDone <-chan struct{}, logger *log.Logger) {
	files := m.secretFiles()
	for _, file := range files {
		if file == "" {
			continue
		}
		m.watchFile(ctxDone, file, logger)
	}
}

func (m *Manager) secretFiles() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return []string{m.handshakeFile, m.passwordFile, m.passwordHashFile}
}

func (m *Manager) watchFile(ctxDone <-chan struct{}, path string, logger *log.Logger) {
	m.watchMu.Lock()
	if _, exists := m.watchers[path]; exists {
		m.watchMu.Unlock()
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		if logger != nil {
			logger.Printf("failed to create watcher for %s: %v", path, err)
		}
		m.watchMu.Unlock()
		return
	}

	if err := ensureDir(path); err != nil {
		if logger != nil {
			logger.Printf("failed to ensure directory for %s: %v", path, err)
		}
		watcher.Close()
		m.watchMu.Unlock()
		return
	}

	if err := watcher.Add(path); err != nil {
		dir := filepath.Dir(path)
		if dir != "" && dir != "." {
			if errDir := watcher.Add(dir); errDir == nil {
				if logger != nil {
					logger.Printf("watching directory %s for changes to %s", dir, path)
				}
			} else {
				if logger != nil {
					logger.Printf("failed to watch %s: %v", path, err)
				}
				watcher.Close()
				m.watchMu.Unlock()
				return
			}
		} else {
			if logger != nil {
				logger.Printf("failed to watch %s: %v", path, err)
			}
			watcher.Close()
			m.watchMu.Unlock()
			return
		}
	}

	m.watchers[path] = watcher
	m.watchMu.Unlock()

	go func() {
		defer func() {
			watcher.Close()
			m.watchMu.Lock()
			delete(m.watchers, path)
			m.watchMu.Unlock()
		}()

		for {
			select {
			case <-ctxDone:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Name != "" && filepath.Clean(event.Name) != filepath.Clean(path) {
					continue
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
					if err := m.Reload(); err != nil {
						if logger != nil {
							logger.Printf("failed to reload secrets after %s change: %v", path, err)
						}
					} else if logger != nil {
						logger.Printf("reloaded secrets after %s change", path)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				if logger != nil {
					logger.Printf("watcher error for %s: %v", path, err)
				}
			}
		}
	}()
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func readSecret(envKey, fileEnv string) (string, string, error) {
	if file := strings.TrimSpace(os.Getenv(fileEnv)); file != "" {
		value, err := readFileSecret(file)
		return value, file, err
	}

	value := strings.TrimSpace(os.Getenv(envKey))
	return value, "", nil
}

func readPassword() (hash string, passwordFile, hashFile string, err error) {
	if file := strings.TrimSpace(os.Getenv("NCM_API_PASSWORD_HASH_FILE")); file != "" {
		value, err := readFileSecret(file)
		return strings.ToLower(value), "", file, err
	}

	if file := strings.TrimSpace(os.Getenv("NCM_API_PASSWORD_FILE")); file != "" {
		value, err := readFileSecret(file)
		if err != nil {
			return "", file, "", err
		}
		return hashPassword(value), file, "", nil
	}

	if value := strings.TrimSpace(os.Getenv("NCM_API_PASSWORD_HASH")); value != "" {
		return strings.ToLower(value), "", "", nil
	}

	if value := strings.TrimSpace(os.Getenv("NCM_API_PASSWORD")); value != "" {
		return hashPassword(value), "", "", nil
	}

	return "", "", "", errors.New("API password not configured")
}

func readFileSecret(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func hashPassword(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) Close() error {
	m.watchMu.Lock()
	defer m.watchMu.Unlock()

	var firstErr error
	for path, watcher := range m.watchers {
		if err := watcher.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close watcher %s: %w", path, err)
		}
		delete(m.watchers, path)
	}
	return firstErr
}
