//go:build linux

package metrics

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

func readDMIField(name string) string {
	data, err := os.ReadFile(filepath.Join("/sys/class/dmi/id", name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func RecordBiosInfo() {
	manufacturer := readDMIField("bios_vendor")
	version := readDMIField("bios_version")
	releaseDate := readDMIField("bios_date")

	if manufacturer == "" && version == "" && releaseDate == "" {
		log.Printf("BIOS info unavailable from /sys/class/dmi/id")
		return
	}

	if releaseDate == "" {
		releaseDate = "unknown"
	}

	BiosInfo.With(prometheus.Labels{
		"manufacturer": manufacturer,
		"version":      version,
		"release_date": releaseDate,
	}).Set(1)
}
