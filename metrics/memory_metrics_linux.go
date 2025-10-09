//go:build linux

package metrics

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/mem"
)

type memoryModule struct {
	SizeBytes    uint64
	Manufacturer string
	PartNumber   string
	SerialNumber string
	Speed        string
}

func RecordMemoryModuleInfo() {
	modules, err := parseMemoryModules()
	if err != nil {
		log.Printf("failed to read memory module inventory: %v", err)
	}

	if len(modules) == 0 {
		stats, err := mem.VirtualMemory()
		if err != nil {
			log.Printf("failed to read virtual memory stats: %v", err)
			return
		}

		capacityGB := float64(stats.Total) / (1024 * 1024 * 1024)
		MemoryModuleInfo.With(prometheus.Labels{
			"capacity":      fmt.Sprintf("%.2fGb", capacityGB),
			"manufacturer":  "unknown",
			"part_number":   "unknown",
			"serial_number": "unknown",
			"speed":         "unknown",
		}).Set(capacityGB)
		return
	}

	for _, module := range modules {
		capacityGB := float64(module.SizeBytes) / (1024 * 1024 * 1024)
		MemoryModuleInfo.With(prometheus.Labels{
			"capacity":      fmt.Sprintf("%.2fGb", capacityGB),
			"manufacturer":  module.Manufacturer,
			"part_number":   module.PartNumber,
			"serial_number": module.SerialNumber,
			"speed":         module.Speed,
		}).Set(capacityGB)
	}
}

func RecordMemoryUsage() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			stats, err := mem.VirtualMemory()
			if err != nil {
				log.Printf("failed to read virtual memory stats: %v", err)
				<-ticker.C
				continue
			}

			TotalMemory.Set(float64(stats.Total))
			UsedMemory.Set(float64(stats.Used))
			FreeMemory.Set(float64(stats.Available))

			<-ticker.C
		}
	}()
}

func parseMemoryModules() ([]memoryModule, error) {
	cmd := exec.Command("dmidecode", "--type", "memory")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var modules []memoryModule
	blocks := strings.Split(string(output), "\n\n")
	for _, block := range blocks {
		lines := strings.Split(block, "\n")
		if len(lines) == 0 {
			continue
		}

		if !strings.Contains(lines[0], "Memory Device") {
			continue
		}

		module := memoryModule{}
		skip := false

		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "Size":
				if strings.EqualFold(value, "No Module Installed") {
					skip = true
					break
				}
				fields := strings.Fields(value)
				if len(fields) >= 2 {
					number, err := strconv.ParseUint(fields[0], 10, 64)
					if err == nil {
						unit := strings.ToUpper(fields[1])
						switch {
						case strings.HasPrefix(unit, "MB"):
							module.SizeBytes = number * 1024 * 1024
						case strings.HasPrefix(unit, "GB"):
							module.SizeBytes = number * 1024 * 1024 * 1024
						}
					}
				}
			case "Manufacturer":
				module.Manufacturer = value
			case "Part Number":
				module.PartNumber = value
			case "Serial Number":
				module.SerialNumber = value
			case "Configured Clock Speed":
				module.Speed = normalizeMemorySpeed(value)
			case "Speed":
				if module.Speed == "" {
					module.Speed = normalizeMemorySpeed(value)
				}
			}
		}

		if skip || module.SizeBytes == 0 {
			continue
		}

		if module.Manufacturer == "" {
			module.Manufacturer = "unknown"
		}
		if module.PartNumber == "" {
			module.PartNumber = "unknown"
		}
		if module.SerialNumber == "" {
			module.SerialNumber = "unknown"
		}
		if module.Speed == "" {
			module.Speed = "unknown"
		}

		modules = append(modules, module)
	}

	return modules, nil
}

func normalizeMemorySpeed(value string) string {
	if value == "" || strings.EqualFold(value, "Unknown") {
		return "unknown"
	}

	fields := strings.Fields(value)
	if len(fields) == 0 {
		return strings.ToLower(value)
	}

	number := fields[0]
	unit := "MHz"
	if len(fields) > 1 {
		unit = fields[1]
	}

	return fmt.Sprintf("%s%s", number, strings.TrimSpace(unit))
}
