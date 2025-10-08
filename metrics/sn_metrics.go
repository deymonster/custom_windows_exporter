package metrics

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"

	"node_exporter_custom/internal/deviceconfig"
)

var (
	SerialNumberMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "device_serial_number_info",
			Help: "Device serial number info and additional info",
		},
		[]string{"serial_number", "location", "device_tag"},
	)
)

func UpdateSerialNumberMetrics(deviceConfig *deviceconfig.Config) {
	if deviceConfig == nil {
		return
	}

	SerialNumberMetric.Reset()
	SerialNumberMetric.With(prometheus.Labels{
		"serial_number": deviceConfig.SerialNumber,
		"location":      deviceConfig.Location,
		"device_tag":    deviceConfig.DeviceTag,
	}).Set(1)
	log.Printf("Metrics updated with new config: %v", deviceConfig)
}

func RecordSNMetrics(config *deviceconfig.Config) {
	if config == nil {
		return
	}

	serialNumber := config.SerialNumber
	if serialNumber == "" {
		serialNumber = "unknown"
		log.Println("Serial number is empty")
	}
	location := config.Location
	if location == "" {
		location = "unknown"
		log.Println("Location is empty")
	}
	deviceTag := config.DeviceTag
	if deviceTag == "" {
		deviceTag = "unknown"
		log.Println("Device tag is empty")
	}

	SerialNumberMetric.Reset()
	SerialNumberMetric.With(prometheus.Labels{
		"serial_number": serialNumber,
		"location":      location,
		"device_tag":    deviceTag,
	}).Set(1)
}
