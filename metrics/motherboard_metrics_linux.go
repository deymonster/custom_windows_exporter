//go:build linux

package metrics

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"
)

func RecordMotherboardInfo() {
	manufacturer := readDMIField("board_vendor")
	product := readDMIField("board_name")
	serial := readDMIField("board_serial")
	version := readDMIField("board_version")

	if manufacturer == "" && product == "" && serial == "" {
		log.Printf("baseboard information unavailable from /sys/class/dmi/id")
	}

	MotherboardInfo.With(prometheus.Labels{
		"manufacturer":  manufacturer,
		"product":       product,
		"serial_number": serial,
		"version":       version,
	}).Set(1)
}
