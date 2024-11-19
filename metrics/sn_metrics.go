package metrics

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"
)

const configFilePath = `C:\ProgramData\NITRINOnetControlManager\config.yml`

type Config struct {
	SerialNumber string `yaml:"SerialNumber"`
	Location     string `yaml:"Location"`
	DeviceTag    string `yaml:"DeviceTag"`
}

func readDeviceConfig() (*Config, error) {

	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Printf("Config file %s does not exist, create one", configFilePath)
		err := createDefaultConfigFile()
		if err != nil {
			return nil, fmt.Errorf("failed to create default config file: %v", err)
		}
	}

	file, err := os.Open(configFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	err = yaml.Unmarshal(byteValue, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file: %v", err)
	}

	return &config, nil
}

func createDefaultConfigFile() error {
	defaultConfig := Config{
		SerialNumber: "unknown",
		Location:     "unknown",
		DeviceTag:    "unknown",
	}

	data, err := yaml.Marshal(&defaultConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %v", err)
	}

	err = os.WriteFile(configFilePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write default config file: %v", err)
	}

	return nil
}

var (
	SerialNumberMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "device_serial_number_info",
			Help: "Device serial number info and additional info",
		},
		[]string{"serial_number", "location", "device_tag"},
	)
)

func RecordSNMetrics() {
	go func() {
		deviceConfig, err := readDeviceConfig()
		if err != nil {
			log.Printf("Error reading device config: %v", err)
			return
		}

		serialNumber := deviceConfig.SerialNumber
		if serialNumber == "" {
			serialNumber = "unknown"
			log.Println("Serial number is empty")
		}
		location := deviceConfig.Location
		if location == "" {
			location = "unknown"
			log.Println("Location is empty")
		}
		deviceTag := deviceConfig.DeviceTag
		if deviceTag == "" {
			deviceTag = "unknown"
			log.Println("Device tag is empty")
		}
		SerialNumberMetric.With(prometheus.Labels{
			"serial_number": serialNumber,
			"location":      location,
			"device_tag":    deviceTag,
		}).Set(1)

	}()
}
