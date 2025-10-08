package mockconfig

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Config struct {
	Enabled              bool    `json:"enabled"`
	UUIDChanged          bool    `json:"uuid_changed"`
	CPULoad              int     `json:"cpu_load"`
	CPUTemperature       float64 `json:"cpu_temperature"`
	DiskFreePercent      int     `json:"disk_free_percent"`
	MemoryFreePercent    int     `json:"memory_free_percent"`
	NetworkErrors        int     `json:"network_errors"`
	DiskReadBytesPerSec  int64   `json:"disk_read_bytes_per_sec"`
	DiskWriteBytesPerSec int64   `json:"disk_write_bytes_per_sec"`
}

var (
	config     Config
	configLock sync.RWMutex
	configPath = "configs/mock_config.json"
)

func Path() string {
	configLock.RLock()
	defer configLock.RUnlock()
	return configPath
}

func SetPath(path string) {
	configLock.Lock()
	defer configLock.Unlock()
	configPath = path
}

func Load() error {
	path := Path()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory for mock config: %v", err)
		}

		defaultConfig := Config{
			Enabled:              false,
			UUIDChanged:          false,
			CPULoad:              50,
			CPUTemperature:       70.0,
			DiskFreePercent:      10,
			MemoryFreePercent:    15,
			NetworkErrors:        10,
			DiskReadBytesPerSec:  1000000,
			DiskWriteBytesPerSec: 500000,
		}

		data, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal default mock config: %v", err)
		}

		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("failed to write default mock config: %v", err)
		}

		configLock.Lock()
		config = defaultConfig
		configLock.Unlock()
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read mock config: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("failed to unmarshal mock config: %v", err)
	}

	configLock.Lock()
	config = loaded
	configLock.Unlock()
	return nil
}

func Watch(ctxDone <-chan struct{}) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Failed to create watcher for mock config: %v", err)
		return
	}
	defer watcher.Close()

	if err := Load(); err != nil {
		log.Printf("Failed to load mock config: %v", err)
	}

	if err := watcher.Add(Path()); err != nil {
		log.Printf("Failed to watch mock config file: %v", err)
		return
	}

	for {
		select {
		case <-ctxDone:
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				time.Sleep(100 * time.Millisecond)
				if err := Load(); err != nil {
					log.Printf("Failed to reload mock config: %v", err)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Error watching mock config file: %v", err)
		}
	}
}

func IsEnabled() bool {
	configLock.RLock()
	defer configLock.RUnlock()
	return config.Enabled
}

func Snapshot() Config {
	configLock.RLock()
	defer configLock.RUnlock()
	return config
}
