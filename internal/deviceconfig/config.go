package deviceconfig

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

type Config struct {
	SerialNumber string `yaml:"serial_number"`
	Location     string `yaml:"location"`
	DeviceTag    string `yaml:"device_tag"`
}

func DefaultPath() string {
	if override := strings.TrimSpace(os.Getenv("NCM_CONFIG_PATH")); override != "" {
		return override
	}

	return defaultConfigPath()
}

func Read(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Printf("Config file %s does not exist, create one", path)
		if err := createDefaultConfigFile(path); err != nil {
			return nil, fmt.Errorf("failed to create default config file: %v", err)
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(byteValue, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file: %v", err)
	}

	return &config, nil
}

func createDefaultConfigFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	defaultConfigContent := "serial_number: \"unknown\"\nlocation: \"unknown\"\ndevice_tag: \"unknown\""

	if err := os.WriteFile(path, []byte(defaultConfigContent), 0o644); err != nil {
		return fmt.Errorf("failed to write default config file: %v", err)
	}

	return nil
}
