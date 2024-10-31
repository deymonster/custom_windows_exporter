package metrics

import (
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

var (
	MotherboardInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "motherboard_info",
			Help: "Motherboard info",
		},
		[]string{"manufacturer", "product", "serial_number", "version"},
	)
)

func RecordMotherboardInfo() {
	var motherboard []Win32_BaseBoard
	err := wmi.Query("SELECT Manufacturer, Product, SerialNumber, Version FROM Win32_BaseBoard", &motherboard)
	if err != nil {
		log.Printf("Error getting motherboard info: %v", err)
		return
	}
	for _, mb := range motherboard {
		MotherboardInfo.With(prometheus.Labels{
			"manufacturer":  mb.Manufacturer,
			"product":       mb.Product,
			"serial_number": mb.SerialNumber,
			"version":       mb.Version,
		}).Set(1)
	}
}
