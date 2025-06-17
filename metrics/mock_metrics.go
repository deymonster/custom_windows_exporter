package metrics

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/prometheus/client_golang/prometheus"
)

// MockConfig содержит конфигурацию для эмуляции метрик
type MockConfig struct {
	// Флаг включения режима эмуляции
	Enabled bool `json:"enabled"`
	// Эмуляция изменения UUID
	UUIDChanged bool `json:"uuid_changed"`
	// Эмуляция загрузки CPU
	CPULoad int `json:"cpu_load"`
	// Эмуляция температуры CPU
	CPUTemperature float64 `json:"cpu_temperature"`
	// Эмуляция свободного места на диске (процент)
	DiskFreePercent int `json:"disk_free_percent"`
	// Эмуляция свободной памяти (процент)
	MemoryFreePercent int `json:"memory_free_percent"`
	// Эмуляция ошибок сетевой карты
	NetworkErrors int `json:"network_errors"`
	// Эмуляция скорости чтения диска (байт/сек)
	DiskReadBytesPerSec int64 `json:"disk_read_bytes_per_sec"`
	// Эмуляция скорости записи диска (байт/сек)
	DiskWriteBytesPerSec int64 `json:"disk_write_bytes_per_sec"`
}

var (
	// Глобальная конфигурация эмуляции
	mockConfig     MockConfig
	mockConfigLock sync.RWMutex
	// Путь к файлу конфигурации эмуляции
	MockConfigPath = "configs/mock_config.json"
)

// LoadMockConfig загружает конфигурацию эмуляции из файла
func LoadMockConfig() error {
	// Проверяем существование файла
	if _, err := os.Stat(MockConfigPath); os.IsNotExist(err) {
		// Создаем директорию, если не существует
		dir := filepath.Dir(MockConfigPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for mock config: %v", err)
		}

		// Создаем файл с дефолтной конфигурацией
		defaultConfig := MockConfig{
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

		if err := os.WriteFile(MockConfigPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write default mock config: %v", err)
		}

		mockConfigLock.Lock()
		mockConfig = defaultConfig
		mockConfigLock.Unlock()
		return nil
	}

	// Читаем файл конфигурации
	data, err := os.ReadFile(MockConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read mock config: %v", err)
	}

	mockConfigLock.Lock()
	defer mockConfigLock.Unlock()

	if err := json.Unmarshal(data, &mockConfig); err != nil {
		return fmt.Errorf("failed to unmarshal mock config: %v", err)
	}

	return nil
}

func WatchMockConfigFile() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Failed to create watcher for mock config: %v", err)
		return
	}
	defer watcher.Close()

	// Загружаем конфиг сразу при старте
	if err := LoadMockConfig(); err != nil {
		log.Printf("Failed to load mock config: %v", err)
	}

	// Добавляем файл для наблюдения
	if err := watcher.Add(MockConfigPath); err != nil {
		log.Printf("Failed to watch mock config file: %v", err)
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				time.Sleep(100 * time.Millisecond) // Небольшая задержка для завершения записи
				log.Println("Mock config file changed, reloading...")
				if err := LoadMockConfig(); err != nil {
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

// IsMockEnabled проверяет, включен ли режим эмуляции
func IsMockEnabled() bool {
	mockConfigLock.RLock()
	defer mockConfigLock.RUnlock()
	return mockConfig.Enabled
}

// ApplyMockMetrics применяет эмулированные метрики
func ApplyMockMetrics() {
	if !IsMockEnabled() {
		return
	}

	log.Println("Applying mock metrics...")

	// Применяем эмулированные метрики
	go func() {
		for {
			mockConfigLock.RLock()
			config := mockConfig
			mockConfigLock.RUnlock()

			// Эмуляция изменения UUID
			if config.UUIDChanged {
				HardwareUUIDChanged.Set(1)
			} else {
				HardwareUUIDChanged.Set(0)
			}

			// Эмуляция загрузки CPU
			CpuUsage.With(prometheus.Labels{
				"core":          "core_0",
				"processor":     "Mock CPU",
				"logical_cores": "4",
			}).Set(float64(config.CPULoad))

			// Эмуляция температуры CPU
			CpuTemperature.With(prometheus.Labels{
				"sensor": "Mock Sensor",
			}).Set(config.CPUTemperature)

			// Эмуляция свободного места на диске
			totalSpace := float64(1000000000000) // 1 TB
			freeSpace := totalSpace * float64(config.DiskFreePercent) / 100.0
			usedSpace := totalSpace - freeSpace
			DiskUsage.With(prometheus.Labels{
				"disk":  "C:",
				"model": "Mock Disk",
				"type":  "SSD",
			}).Set(usedSpace)

			DiskUsagePercent.With(prometheus.Labels{
				"disk":  "C:",
				"model": "Mock Disk",
			}).Set(100.0 - float64(config.DiskFreePercent))

			// Эмуляция скорости чтения/записи диска
			DiskReadBytes.With(prometheus.Labels{
				"disk":  "C:",
				"model": "Mock Disk",
			}).Set(float64(config.DiskReadBytesPerSec))

			DiskWriteBytes.With(prometheus.Labels{
				"disk":  "C:",
				"model": "Mock Disk",
			}).Set(float64(config.DiskWriteBytesPerSec))

			// Эмуляция свободной памяти
			totalMemory := float64(16000000000) // 16 GB
			freeMemory := totalMemory * float64(config.MemoryFreePercent) / 100.0
			usedMemory := totalMemory - freeMemory

			TotalMemory.Set(totalMemory)
			FreeMemory.Set(freeMemory)
			UsedMemory.Set(usedMemory)

			// Эмуляция ошибок сетевой карты
			NetworkErrors.With(prometheus.Labels{
				"interface": "Mock Ethernet",
			}).Add(float64(config.NetworkErrors))

			time.Sleep(5 * time.Second)
		}
	}()
}
