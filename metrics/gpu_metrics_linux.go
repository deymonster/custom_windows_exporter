//go:build linux

package metrics

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

func readSysfsValue(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func discoverGPUNames() []string {
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return nil
	}

	var names []string
	seen := make(map[string]struct{})

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "card") || strings.Contains(name, "-") {
			continue
		}

		deviceDir := filepath.Join("/sys/class/drm", name, "device")
		vendor := readSysfsValue(filepath.Join(deviceDir, "vendor"))
		model := readSysfsValue(filepath.Join(deviceDir, "device"))

		driver := ""
		if link, err := os.Readlink(filepath.Join(deviceDir, "driver")); err == nil {
			driver = filepath.Base(link)
		}

		// Some kernels expose vendor/model as hex values (e.g. 0x10de)
		identifier := fmt.Sprintf("%s:%s", vendor, model)
		if vendor == "" && model == "" {
			identifier = name
		}
		if driver != "" {
			identifier = fmt.Sprintf("%s (%s)", identifier, driver)
		}

		if _, ok := seen[identifier]; ok {
			continue
		}
		seen[identifier] = struct{}{}
		names = append(names, identifier)
	}

	return names
}

func RecordGpuInfo() {
	names := discoverGPUNames()
	if len(names) == 0 {
		log.Printf("no GPU entries found under /sys/class/drm; exposing placeholder metric")
		names = []string{"unknown"}
	}

	for _, name := range names {
		GpuInfo.With(prometheus.Labels{"name": name}).Set(1)
		// Memory information is not universally available; publish zero if unknown.
		GpuMemory.With(prometheus.Labels{"name": name}).Set(0)
	}
}
