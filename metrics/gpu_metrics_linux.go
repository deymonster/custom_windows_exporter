//go:build linux

package metrics

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

type gpuDevice struct {
	Name        string
	MemoryBytes uint64
	Type        string
}

func RecordGpuInfo() {
	devices := discoverGPUDevices()
	if len(devices) == 0 {
		log.Printf("no GPU entries found under /sys/class/drm; exposing placeholder metric")
		devices = []gpuDevice{{Name: "unknown", MemoryBytes: 0, Type: "unknown"}}
	}

	for _, device := range devices {
		GpuInfo.With(prometheus.Labels{"name": device.Name}).Set(1)
		GpuMemory.With(prometheus.Labels{"name": device.Name}).Set(float64(device.MemoryBytes))
		GpuType.With(prometheus.Labels{"name": device.Name, "type": device.Type}).Set(1)
	}
}

// определение устройств c типом
func discoverGPUDevices() []gpuDevice {
    busNames := parseLspciGPUInfo()
    nvidiaMemory := queryNvidiaSMIMemory()

    entries, err := os.ReadDir("/sys/class/drm")
    if err != nil {
        return nil
    }

    var devices []gpuDevice
    seen := make(map[string]struct{})

    for _, entry := range entries {
        name := entry.Name()
        if !strings.HasPrefix(name, "card") || strings.Contains(name, "-") {
            continue
        }

        deviceDir := filepath.Join("/sys/class/drm", name, "device")
        resolved, err := filepath.EvalSymlinks(deviceDir)
        if err != nil {
            continue
        }

        busID := normalizePCIBusID(filepath.Base(resolved))
        if busID == "" {
            continue
        }

        if _, ok := seen[busID]; ok {
            continue
        }
        seen[busID] = struct{}{}

        label := busNames[busID]
        if label == "" {
            vendor := strings.TrimPrefix(readSysfsValue(filepath.Join(deviceDir, "vendor")), "0x")
            device := strings.TrimPrefix(readSysfsValue(filepath.Join(deviceDir, "device")), "0x")
            label = strings.TrimSpace(fmt.Sprintf("PCI %s:%s", vendor, device))
            if label == "" {
                label = name
            }
        }

        memory := gpuMemoryFromSysfs(deviceDir)
        if memory == 0 {
            if value, ok := nvidiaMemory[busID]; ok {
                memory = value
            }
        }
        if memory == 0 {
            memory = gpuMemoryFromLspci(busID)
        }

        typ := classifyGPUType(deviceDir, label)
        devices = append(devices, gpuDevice{Name: label, MemoryBytes: memory, Type: typ})
    }

    if len(devices) == 0 && len(busNames) > 0 {
        for _, name := range busNames {
            devices = append(devices, gpuDevice{Name: name, MemoryBytes: 0, Type: "unknown"})
        }
    }

    return devices
}

func parseLspciGPUInfo() map[string]string {
	cmd := exec.Command("lspci", "-vmm", "-d", "::0300")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]string{}
	}

	entries := strings.Split(strings.TrimSpace(string(output)), "\n\n")
	info := make(map[string]string, len(entries))

	for _, block := range entries {
		lines := strings.Split(block, "\n")
		var (
			slot    string
			vendor  string
			device  string
			sVendor string
			sDevice string
		)

		for _, line := range lines {
			parts := strings.SplitN(line, ":\t", 2)
			if len(parts) != 2 {
				continue
			}

			switch parts[0] {
			case "Slot":
				slot = strings.TrimSpace(parts[1])
			case "Vendor":
				vendor = strings.TrimSpace(parts[1])
			case "Device":
				device = strings.TrimSpace(parts[1])
			case "SVendor":
				sVendor = strings.TrimSpace(parts[1])
			case "SDevice":
				sDevice = strings.TrimSpace(parts[1])
			}
		}

		if slot == "" {
			continue
		}

		nameParts := []string{}
		if vendor != "" {
			nameParts = append(nameParts, vendor)
		}
		if device != "" {
			nameParts = append(nameParts, device)
		}
		if len(nameParts) == 0 {
			continue
		}

		name := strings.Join(nameParts, " ")
		sub := strings.TrimSpace(strings.Join([]string{sVendor, sDevice}, " "))
		if sub != "" {
			name = fmt.Sprintf("%s (%s)", name, sub)
		}

		info[normalizePCIBusID(slot)] = name
	}

	return info
}

func queryNvidiaSMIMemory() map[string]uint64 {
	cmd := exec.Command("nvidia-smi", "--query-gpu=pci.bus_id,memory.total", "--format=csv,noheader,nounits")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]uint64{}
	}

	memory := make(map[string]uint64)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}

		busID := normalizePCIBusID(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[len(parts)-1])
		if busID == "" || value == "" {
			continue
		}

		if miB, err := strconv.ParseUint(value, 10, 64); err == nil {
			memory[busID] = miB * 1024 * 1024
		}
	}

	return memory
}

func gpuMemoryFromSysfs(deviceDir string) uint64 {
	candidates := []string{
		"mem_info_vram_total",
		"mem_info_vis_vram_total",
		"mem_info_dedicated_total",
		"mem_info_gtt_total",
		"total_vram",
		"vram_total",
		"local_memory_size",
	}

	for _, candidate := range candidates {
		value := readSysfsValue(filepath.Join(deviceDir, candidate))
		if value == "" {
			continue
		}

		if bytes, ok := parseNumericValue(value); ok {
			return bytes
		}
	}

	return 0
}

func gpuMemoryFromLspci(busID string) uint64 {
	busID = strings.TrimSpace(busID)
	if busID == "" {
		return 0
	}

	variants := []string{busID}
	if strings.HasPrefix(busID, "0000:") {
		variants = append(variants, strings.TrimPrefix(busID, "0000:"))
	}

	var output []byte
	var err error

	for _, variant := range variants {
		cmd := exec.Command("lspci", "-v", "-s", variant)
		output, err = cmd.CombinedOutput()
		if err == nil {
			break
		}
	}

	if err != nil {
		return 0
	}

	maxBytes := uint64(0)
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.Contains(line, "[size=") {
			continue
		}

		start := strings.Index(line, "[size=")
		if start < 0 {
			continue
		}

		segment := line[start+6:]
		end := strings.Index(segment, "]")
		if end < 0 {
			continue
		}

		token := segment[:end]
		if bytes := parseSizeToken(token); bytes > maxBytes {
			maxBytes = bytes
		}
	}

	return maxBytes
}

func parseNumericValue(raw string) (uint64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}

	if strings.HasPrefix(trimmed, "0x") || strings.HasPrefix(trimmed, "0X") {
		value, err := strconv.ParseUint(trimmed[2:], 16, 64)
		if err == nil {
			return value, true
		}
	}

	if value, err := strconv.ParseUint(trimmed, 10, 64); err == nil {
		return value, true
	}

	return 0, false
}

func parseSizeToken(token string) uint64 {
	cleaned := strings.TrimSpace(strings.TrimSuffix(token, "B"))
	if cleaned == "" {
		return 0
	}

	cleaned = strings.ToUpper(cleaned)

	unit := ""
	for len(cleaned) > 0 {
		last := cleaned[len(cleaned)-1]
		if last >= '0' && last <= '9' || last == '.' {
			break
		}
		unit = string(last) + unit
		cleaned = cleaned[:len(cleaned)-1]
	}

	if cleaned == "" {
		return 0
	}

	number, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0
	}

	multiplier := float64(1)
	switch unit {
	case "K", "KI", "KIB", "KB":
		multiplier = 1024
	case "M", "MI", "MIB", "MB":
		multiplier = 1024 * 1024
	case "G", "GI", "GIB", "GB":
		multiplier = 1024 * 1024 * 1024
	case "T", "TI", "TIB", "TB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		multiplier = 1
	}

	return uint64(number * multiplier)
}

func normalizePCIBusID(id string) string {
	trimmed := strings.ToLower(strings.TrimSpace(id))
	if trimmed == "" {
		return ""
	}

	if strings.Count(trimmed, ":") == 1 {
		trimmed = "0000:" + trimmed
	}

	return trimmed
}

// функции классификации
func classifyGPUType(deviceDir, label string) string {
    vendorHex := strings.TrimPrefix(readSysfsValue(filepath.Join(deviceDir, "vendor")), "0x")
    v := strings.ToLower(strings.TrimSpace(vendorHex))
    name := strings.ToLower(label)

    if v == "8086" || strings.Contains(name, "intel") {
        return "integrated"
    }
    if v == "10de" || strings.Contains(name, "nvidia") {
        return "discrete"
    }
    if v == "1002" || strings.Contains(name, "amd") || strings.Contains(name, "radeon") {
        if hasDedicatedMemory(deviceDir) {
            return "discrete"
        }
        return "integrated"
    }

    if hasDedicatedMemory(deviceDir) {
        return "discrete"
    }
    return "unknown"
}

func hasDedicatedMemory(deviceDir string) bool {
    // Наличие файлов про выделенную VRAM часто у дискретных адаптеров
    candidates := []string{
        "mem_info_vram_total",
        "mem_info_vis_vram_total",
        "mem_info_dedicated_total",
        "total_vram",
        "vram_total",
        "local_memory_size",
    }
    for _, c := range candidates {
        val := readSysfsValue(filepath.Join(deviceDir, c))
        if val == "" {
            continue
        }
        if bytes, ok := parseNumericValue(val); ok && bytes > 0 {
            return true
        }
    }
    return false
}
