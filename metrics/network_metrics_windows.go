//go:build windows

package metrics

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/StackExchange/wmi"
	"github.com/shirou/gopsutil/net"
)

type Win32_NetworkAdapter struct {
	Name            string
	MACAddress      string
	Manufacturer    string
	NetEnabled      bool
	PNPDeviceID     string
	Description     string
	NetConnectionID string
}

// GetPhysicalNetworkAdapters retrieves information about physical network adapters in the system
// by querying the Win32_NetworkAdapter WMI class. It filters out virtual adapters and adapters
// without a MAC address. It returns a slice of Win32_NetworkAdapter structs containing details
// such as the name, MAC address, manufacturer, and enabled status of each physical adapter, or
// an error if the query fails.
func GetPhysicalNetworkAdapters() ([]Win32_NetworkAdapter, error) {
	var adapters []Win32_NetworkAdapter
	err := wmi.Query(
		`SELECT Name, MACAddress, Manufacturer, NetEnabled, PNPDeviceID, 
		Description, NetConnectionID FROM Win32_NetworkAdapter 
		WHERE MACAddress IS NOT NULL AND PhysicalAdapter = TRUE`,
		&adapters,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get network adapters: %v", err)
	}

	var physicalAdapters []Win32_NetworkAdapter
	for _, adapter := range adapters {
		if adapter.Manufacturer == "Microsoft" ||
			strings.Contains(adapter.PNPDeviceID, "ROOT\\") ||
			strings.Contains(adapter.Description, "Virtual") ||
			strings.Contains(adapter.Name, "WAN Miniport") {
			continue
		}
		physicalAdapters = append(physicalAdapters, adapter)
	}

	if len(physicalAdapters) == 0 {
		return nil, fmt.Errorf("no physical network adapters found")
	}

	return physicalAdapters, nil
}

// GetNetworkIOStats retrieves network I/O statistics for all interfaces on the system.
// It returns a slice of net.IOCountersStat structs containing the I/O statistics for
// each interface, or an error if the query fails.
func GetNetworkIOStats() ([]net.IOCountersStat, error) {
	stats, err := net.IOCounters(true)
	if err != nil {
		return nil, fmt.Errorf("failed to get network stats: %v", err)
	}
	return stats, nil
}

// RecordNetworkAdapterStatus updates the Prometheus metric for network status
// based on the enabled state of each network adapter. It sets the metric to 1.0
// if the adapter is enabled and 0.0 if it is disabled. The metric is labeled
// with the name of the network interface.

func RecordNetworkAdapterStatus(adapters []Win32_NetworkAdapter) {
	for _, adapter := range adapters {
		status := 0.0
		if adapter.NetEnabled {
			status = 1.0
		}
		NetworkStatus.With(prometheus.Labels{
			"interface": adapter.Name,
		}).Set(status)
	}
}

// RecordNetworkTraffic records network traffic metrics based on the given
// previous and current network statistics. It records the following metrics
// for each network interface:
//
// * NetworkRxBytesPerSecond: The number of bytes received per second
// * NetworkTxBytesPerSecond: The number of bytes sent per second
// * NetworkErrors: The total number of errors (inbound and outbound)
// * NetworkDroppedPackets: The total number of dropped packets (inbound and outbound)
//
// It returns a new map of current network statistics for the next call.
func RecordNetworkTraffic(prevStats map[string]net.IOCountersStat, currentStats []net.IOCountersStat, adapterMap map[string]string) map[string]net.IOCountersStat {
	newStats := make(map[string]net.IOCountersStat)

	for _, stat := range currentStats {
		if name, ok := adapterMap[stat.Name]; ok {
			if prev, exists := prevStats[stat.Name]; exists {
				duration := 5.0
				rxRate := float64(stat.BytesRecv-prev.BytesRecv) / duration
				txRate := float64(stat.BytesSent-prev.BytesSent) / duration

				NetworkRxBytesPerSecond.With(prometheus.Labels{"interface": name}).Set(rxRate)
				NetworkTxBytesPerSecond.With(prometheus.Labels{"interface": name}).Set(txRate)

				NetworkErrors.With(prometheus.Labels{"interface": name}).Add(float64(stat.Errin + stat.Errout))
				NetworkDroppedPackets.With(prometheus.Labels{"interface": name}).Add(float64(stat.Dropin + stat.Dropout))
			}
			newStats[stat.Name] = stat
		}
	}

	return newStats
}

// RecordNetworkMetrics runs in a separate goroutine and updates the network metrics
// every 5 seconds. It records the status of physical network adapters and their
// network traffic metrics.
func RecordNetworkMetrics() {
	go func() {
		prevStats := make(map[string]net.IOCountersStat)

		for {
			// Получение только физических сетевых адаптеров через WMI
			adapters, err := GetPhysicalNetworkAdapters()
			if err != nil {
				log.Printf("%v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// Создание маппинга интерфейсов
			adapterMap := make(map[string]string)
			for _, adapter := range adapters {
				adapterMap[adapter.NetConnectionID] = adapter.Name
			}

			// Запись статуса адаптеров
			RecordNetworkAdapterStatus(adapters)

			// Получение статистики
			ioStats, err := GetNetworkIOStats()
			if err != nil {
				log.Printf("%v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// Запись метрик трафика
			prevStats = RecordNetworkTraffic(prevStats, ioStats, adapterMap)

			time.Sleep(5 * time.Second)
		}
	}()
}
