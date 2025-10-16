//go:build linux

package metrics

import (
	"errors"
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
	var modules []memoryModule
	var errs []error

	if dmidecode, err := parseMemoryModulesFromDmidecode(); err == nil && len(dmidecode) > 0 {
		modules = dmidecode
	} else {
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(modules) == 0 {
		if lshw, err := parseMemoryModulesFromLshw(); err == nil && len(lshw) > 0 {
			modules = lshw
		} else if err != nil {
			errs = append(errs, err)
		}
	}

	if len(modules) == 0 && len(errs) > 0 {
		return modules, errors.Join(errs...)
	}

	return modules, nil
}

func parseMemoryModulesFromDmidecode() ([]memoryModule, error) {
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
				module.Manufacturer = sanitizeMemoryField(value)
			case "Part Number":
				module.PartNumber = sanitizeMemoryField(value)
			case "Serial Number":
				module.SerialNumber = sanitizeMemoryField(value)
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

		if module.Speed == "" {
			module.Speed = "unknown"
		}

		modules = append(modules, module)
	}

	return modules, nil
}

func parseMemoryModulesFromLshw() ([]memoryModule, error) {
	cmd := exec.Command("lshw", "-class", "memory")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var modules []memoryModule
	lines := strings.Split(string(output), "\n")
	module := memoryModule{}
	inBank := false

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "*-bank") || strings.HasPrefix(line, "bank:") {
			if module.SizeBytes > 0 {
				if module.Speed == "" {
					module.Speed = "unknown"
				}
				modules = append(modules, module)
			}
			module = memoryModule{}
			inBank = true
			continue
		}

		if !inBank {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch strings.ToLower(key) {
		case "size":
			size, ok := parseMemoryCapacity(value)
			if ok {
				module.SizeBytes = size
			}
		case "vendor":
			module.Manufacturer = sanitizeMemoryField(value)
		case "product":
			module.PartNumber = sanitizeMemoryField(value)
		case "serial":
			module.SerialNumber = sanitizeMemoryField(value)
		case "clock":
			module.Speed = normalizeMemorySpeed(value)
		}
	}

	if module.SizeBytes > 0 {
		if module.Speed == "" {
			module.Speed = "unknown"
		}
		modules = append(modules, module)
	}

	return modules, nil
}

func parseMemoryCapacity(value string) (uint64, bool) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return 0, false
	}

	number, err := strconv.ParseFloat(strings.TrimSuffix(fields[0], "GiB"), 64)
	if err == nil && strings.Contains(strings.ToLower(value), "gib") {
		return uint64(number * 1024 * 1024 * 1024), true
	}

	number, err = strconv.ParseFloat(strings.TrimSuffix(fields[0], "MiB"), 64)
	if err == nil && strings.Contains(strings.ToLower(value), "mib") {
		return uint64(number * 1024 * 1024), true
	}

	rawNumber, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil || len(fields) < 2 {
		return 0, false
	}

	unit := strings.ToUpper(fields[1])
	switch {
	case strings.HasPrefix(unit, "MB"):
		return rawNumber * 1024 * 1024, true
	case strings.HasPrefix(unit, "GB"):
		return rawNumber * 1024 * 1024 * 1024, true
	}

	return 0, false
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

func sanitizeMemoryField(value string) string {
	cleaned := strings.Trim(value, "\r\n\t")
	if cleaned == "" {
		return "unknown"
	}

	trimmed := strings.TrimSpace(cleaned)
	if trimmed == "" {
		return cleaned
	}

	lower := strings.ToLower(trimmed)
	switch lower {
	case "unknown", "none", "n/a", "not specified", "no module installed":
		return "unknown"
	}

	return trimmed
}
