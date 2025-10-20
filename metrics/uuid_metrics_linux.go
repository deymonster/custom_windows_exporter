//go:build linux

package metrics

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"node_exporter_custom/registryutil"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	gnet "github.com/shirou/gopsutil/net"
)

func collectDiskIdentifiers() []string {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil
	}

	var identifiers []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "fd") {
			continue
		}

		deviceDir := filepath.Join("/sys/block", name, "device")
		model := readSysfsValue(filepath.Join(deviceDir, "model"))
		serial := readSysfsValue(filepath.Join(deviceDir, "serial"))
		sizeSectors := readSysfsValue(filepath.Join("/sys/block", name, "size"))

		sizeBytes := ""
		if sizeSectors != "" {
			if sectors, err := strconv.ParseUint(sizeSectors, 10, 64); err == nil {
				sizeBytes = fmt.Sprintf("%d", sectors*512)
			}
		}

		identifiers = append(identifiers, fmt.Sprintf("%s|%s|%s|%s", name, model, serial, sizeBytes))
	}

	return identifiers
}

func collectMemorySummary() string {
	stats, err := mem.VirtualMemory()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("total=%d,free=%d", stats.Total, stats.Free)
}

func GenerateHardwareUUID() (string, error) {
	var sb strings.Builder

	biosVendor := readDMIField("bios_vendor")
	biosVersion := readDMIField("bios_version")
	biosDate := readDMIField("bios_date")
	if biosVendor == "" && biosVersion == "" {
		return "", fmt.Errorf("bios information unavailable")
	}
	sb.WriteString(fmt.Sprintf("%s|%s|%s", biosVendor, biosVersion, biosDate))

	cpuInfo, err := cpu.Info()
	if err != nil || len(cpuInfo) == 0 {
		return "", fmt.Errorf("cpu info unavailable: %v", err)
	}
	firstCPU := cpuInfo[0]
	sb.WriteString(fmt.Sprintf("|%s|%d", firstCPU.ModelName, firstCPU.Cores))

	// диски
	for _, disk := range collectDiskIdentifiers() {
		sb.WriteString("|")
		sb.WriteString(disk)
	}

	// система
	if hostInfo, err := host.Info(); err == nil {
		sb.WriteString(fmt.Sprintf("|%s|%s|%s", hostInfo.Hostname, hostInfo.Platform, hostInfo.PlatformVersion))
	}

	// MAC физического интерфейса
	if mac := firstPhysicalMAC(); mac != "" {
		sb.WriteString("|" + mac)
	}

	// материнская плата
	baseboardVendor := readDMIField("board_vendor")
	baseboardProduct := readDMIField("board_name")
	baseboardSerial := readDMIField("board_serial")
	baseboardVersion := readDMIField("board_version")
	sb.WriteString(fmt.Sprintf("|%s|%s|%s|%s", baseboardVendor, baseboardProduct, baseboardSerial, baseboardVersion))

	// модули памяти (как в Windows)
	if modules, err := parseMemoryModules(); err == nil && len(modules) > 0 {
		for _, m := range modules {
			sb.WriteString(fmt.Sprintf("|%s|%s|%s|%d|%s",
				m.Manufacturer, m.PartNumber, m.SerialNumber, m.SizeBytes, m.Speed))
		}
	}

	// GPU имя + объём
	for _, gpu := range discoverGPUDevices() {
		sb.WriteString(fmt.Sprintf("|%s|%d", gpu.Name, gpu.MemoryBytes))
	}

	hash := sha256.Sum256([]byte(sb.String()))
	hashStr := hex.EncodeToString(hash[:])

	uuid := fmt.Sprintf("%s-%s-%s-%s-%s",
		hashStr[0:8],
		hashStr[8:12],
		"4"+hashStr[13:16],
		"8"+hashStr[17:20],
		hashStr[20:32],
	)

	return uuid, nil
}

func RecordUUIDMetrics() {
	go func() {
		if err := RefreshUUIDMetrics(); err != nil {
			log.Printf("Failed to record UUID metrics: %v", err)
		}
	}()
}

func RefreshUUIDMetrics() error {
	currentUUID, err := GenerateHardwareUUID()
	if err != nil {
		return fmt.Errorf("generate hardware UUID: %w", err)
	}

	exists, err := registryutil.KeyExists()
	if err != nil {
		return fmt.Errorf("check UUID persistence: %w", err)
	}

	if !exists {
		if err := registryutil.CreateKey(); err != nil {
			return fmt.Errorf("init UUID storage: %w", err)
		}
		if err := registryutil.WriteUUIDToRegistry(currentUUID); err != nil {
			return fmt.Errorf("store hardware UUID: %w", err)
		}
		HardwareUUIDChanged.Set(0)
	} else {
		storedUUID, err := registryutil.ReadUUIDFromRegistry()
		if err != nil {
			return fmt.Errorf("read stored UUID: %w", err)
		}
		if storedUUID != currentUUID {
			HardwareUUIDChanged.Set(1)
			log.Printf("hardware UUID changed: old=%s new=%s", storedUUID, currentUUID)
		} else {
			HardwareUUIDChanged.Set(0)
		}
	}

	SystemUUID.Reset()
	SystemUUID.With(prometheus.Labels{"uuid": currentUUID}).Set(1)
	return nil
}

// helper: firstPhysicalMAC
func firstPhysicalMAC() string {
    ifaces, err := gnet.Interfaces()
    if err != nil {
        return ""
    }
    for _, iface := range ifaces {
        if iface.HardwareAddr == "" || iface.Name == "" || strings.HasPrefix(iface.Name, "lo") {
            continue
        }
        // физический интерфейс обычно имеет привязку к PCI: наличие /sys/class/net/<iface>/device
        devPath := filepath.Join("/sys/class/net", iface.Name, "device")
        if _, err := os.Stat(devPath); err == nil {
            return iface.HardwareAddr
        }
    }
    // fallback: первый любой MAC
    for _, iface := range ifaces {
        if iface.HardwareAddr != "" {
            return iface.HardwareAddr
        }
    }
    return ""
}
