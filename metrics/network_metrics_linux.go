//go:build linux

package metrics

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/net"
)

var (
	ifaceNameCache       map[string]string
	ifaceNameCacheExpiry time.Time
	ifaceNameMu          sync.Mutex
)

func RecordNetworkMetrics() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		prevStats := make(map[string]trackedInterfaceStat)

		for {
			interfaces, err := net.Interfaces()
			if err != nil {
				log.Printf("failed to list network interfaces: %v", err)
				<-ticker.C
				continue
			}

			displayNames := make(map[string]string, len(interfaces))
			for _, iface := range interfaces {
				if iface.Name == "" {
					continue
				}

				display := friendlyInterfaceName(iface.Name)
				displayNames[iface.Name] = display

				status := 0.0
				for _, flag := range iface.Flags {
					if flag == "up" {
						status = 1.0
						break
					}
				}

				NetworkStatus.With(prometheus.Labels{"interface": display}).Set(status)
			}

			stats, err := net.IOCounters(true)
			if err != nil {
				log.Printf("failed to read network counters: %v", err)
				<-ticker.C
				continue
			}

			for _, stat := range stats {
				display := displayNames[stat.Name]
				if display == "" {
					display = friendlyInterfaceName(stat.Name)
					displayNames[stat.Name] = display
					NetworkStatus.With(prometheus.Labels{"interface": display}).Set(0)
				}

				labels := prometheus.Labels{"interface": display}
				if prev, ok := prevStats[stat.Name]; ok {
					elapsed := time.Since(prev.Timestamp).Seconds()
					if elapsed <= 0 {
						elapsed = 5
					}

					rxRate := float64(stat.BytesRecv-prev.Counter.BytesRecv) / elapsed
					txRate := float64(stat.BytesSent-prev.Counter.BytesSent) / elapsed

					if rxRate < 0 {
						rxRate = 0
					}
					if txRate < 0 {
						txRate = 0
					}

					NetworkRxBytesPerSecond.With(labels).Set(rxRate)
					NetworkTxBytesPerSecond.With(labels).Set(txRate)

					errDelta := (stat.Errin + stat.Errout) - (prev.Counter.Errin + prev.Counter.Errout)
					dropDelta := (stat.Dropin + stat.Dropout) - (prev.Counter.Dropin + prev.Counter.Dropout)
					if errDelta < 0 {
						errDelta = 0
					}
					if dropDelta < 0 {
						dropDelta = 0
					}

					NetworkErrors.With(labels).Add(float64(errDelta))
					NetworkDroppedPackets.With(labels).Add(float64(dropDelta))
				} else {
					NetworkRxBytesPerSecond.With(labels).Set(0)
					NetworkTxBytesPerSecond.With(labels).Set(0)
					NetworkErrors.With(labels).Add(0)
					NetworkDroppedPackets.With(labels).Add(0)
				}

				prevStats[stat.Name] = trackedInterfaceStat{Counter: stat, Timestamp: time.Now()}
			}

			<-ticker.C
		}
	}()
}

type trackedInterfaceStat struct {
	Counter   net.IOCountersStat
	Timestamp time.Time
}

func friendlyInterfaceName(iface string) string {
	ifaceNameMu.Lock()
	defer ifaceNameMu.Unlock()

	if time.Now().After(ifaceNameCacheExpiry) {
		ifaceNameCache = loadInterfaceNames()
		ifaceNameCacheExpiry = time.Now().Add(5 * time.Minute)
	}

	if name, ok := ifaceNameCache[iface]; ok && name != "" {
		return name
	}

	return iface
}

func loadInterfaceNames() map[string]string {
	names := make(map[string]string)

	pciNames := loadPCINetworkNames()

	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return names
	}

	for _, entry := range entries {
		iface := entry.Name()
		devicePath := filepath.Join("/sys/class/net", iface, "device")
		resolved, err := filepath.EvalSymlinks(devicePath)
		if err != nil {
			continue
		}

		busID := filepath.Base(resolved)
		if label, ok := pciNames[busID]; ok {
			names[iface] = label
			continue
		}

		vendor := strings.TrimPrefix(readSysfsValue(filepath.Join(resolved, "vendor")), "0x")
		device := strings.TrimPrefix(readSysfsValue(filepath.Join(resolved, "device")), "0x")
		if vendor != "" && device != "" {
			names[iface] = fmt.Sprintf("PCI %s:%s", vendor, device)
		}
	}

	return names
}

func loadPCINetworkNames() map[string]string {
	names := make(map[string]string)

	cmd := exec.Command("lspci", "-Dvmm")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return names
	}

	blocks := strings.Split(strings.TrimSpace(string(output)), "\n\n")
	for _, block := range blocks {
		lines := strings.Split(block, "\n")
		var (
			slot    string
			class   string
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
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			switch key {
			case "Slot":
				slot = value
			case "Class":
				class = value
			case "Vendor":
				vendor = value
			case "Device":
				device = value
			case "SVendor":
				sVendor = value
			case "SDevice":
				sDevice = value
			}
		}

		if slot == "" {
			continue
		}

		classLower := strings.ToLower(class)
		if !strings.Contains(classLower, "ethernet") && !strings.Contains(classLower, "network") {
			continue
		}

		label := strings.TrimSpace(fmt.Sprintf("%s %s", vendor, device))
		if sVendor != "" && sDevice != "" {
			label = strings.TrimSpace(fmt.Sprintf("%s %s", sVendor, sDevice))
		}

		if label == "" {
			label = vendor
		}

		if label != "" {
			names[slot] = label
		}
	}

	return names
}
