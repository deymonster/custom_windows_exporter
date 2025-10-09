//go:build windows

package metrics

import (
	"fmt"
	"log"

	"github.com/StackExchange/wmi"
	"github.com/prometheus/client_golang/prometheus"
)

type Win32_BaseBoard struct {
	Manufacturer string
	Product      string
	SerialNumber string
	Version      string
}

// GetMotherboardInfo retrieves information about the motherboard in the system
// by querying the Win32_BaseBoard WMI class. It returns a slice of
// Win32_BaseBoard structs containing details such as the manufacturer, product,
// serial number, and version of the motherboard. If the query fails, it returns
// an error.
func GetMotherboardInfo() (Win32_BaseBoard, error) {
	var baseBoards []Win32_BaseBoard
	err := wmi.Query("SELECT Manufacturer, Product, SerialNumber, Version FROM Win32_BaseBoard", &baseBoards)
	if err != nil {
		return Win32_BaseBoard{}, fmt.Errorf("failed to get motherboard info: %v", err)
	}

	if len(baseBoards) == 0 {
		return Win32_BaseBoard{}, fmt.Errorf("no motherboard information found")
	}

	return baseBoards[0], nil
}

// RecordMotherboardInfo records information about the motherboard in the system
// to Prometheus. It runs in a separate goroutine and updates the metrics every
// 5 seconds. It is designed to be run as a goroutine in a loop.
func RecordMotherboardInfo() {
	mb, err := GetMotherboardInfo()
	if err != nil {
		log.Printf("Error getting motherboard info: %v", err)
		return
	}
	MotherboardInfo.With(prometheus.Labels{
		"manufacturer":  mb.Manufacturer,
		"product":       mb.Product,
		"serial_number": mb.SerialNumber,
		"version":       mb.Version,
	}).Set(1)
}
