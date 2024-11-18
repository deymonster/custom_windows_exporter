package metrics

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/StackExchange/wmi"
	"github.com/prometheus/client_golang/prometheus"
)

// Структуры для хранения информации о системе и операционной системе
type Win32_ComputerSystem_UUID struct {
	Manufacturer string
	Model        string
}

type Win32_NetworkAdapter struct {
	MACAddress string
}

var (
	SystemUUID = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "UNIQUE_ID_SYSTEM",
			Help: "Unique ID for the system",
		},
		[]string{"uuid"},
	)
)

// Функция для получения аппаратных данных и генерации UUID
func RecordUUIDMetrics() {
	go func() {
		var computerSystem []Win32_ComputerSystem_UUID
		var networkAdapters []Win32_NetworkAdapter

		err := wmi.Query("SELECT Manufacturer, Model FROM Win32_ComputerSystem", &computerSystem)
		if err != nil || len(computerSystem) == 0 {
			log.Printf("Error getting computer system info: %v", err)
			return
		}

		err = wmi.Query("SELECT MACAddress, Manufacturer, NetEnabled, PNPDeviceID FROM Win32_NetworkAdapter WHERE MACAddress IS NOT NULL AND PhysicalAdapter = TRUE", &networkAdapters)
		if err != nil || len(networkAdapters) == 0 {
			log.Printf("Error getting network adapter info: %v", err)
			return
		}

		cs := computerSystem[0]
		macAddress := networkAdapters[0].MACAddress

		combinedInfo := fmt.Sprintf("%s %s %s", cs.Manufacturer, cs.Model, macAddress)
		hash := sha256.Sum256([]byte(combinedInfo))
		uuid := hex.EncodeToString(hash[:])

		SystemUUID.With(prometheus.Labels{
			"uuid": uuid,
		}).Set(1)

	}()
}
