package metrics

import (
	"log"
	"strings"
	"time"

	"github.com/StackExchange/wmi"
	"github.com/prometheus/client_golang/prometheus"
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

// Определение метрик
var (
	NetworkStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_status",
			Help: "Network status on system (1 = up, 0 = down)",
		},
		[]string{"interface"},
	)

	NetworkRxBytesPerSecond = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_rx_bytes_per_second",
			Help: "Incoming network traffic in bytes per second",
		},
		[]string{"interface"},
	)

	NetworkTxBytesPerSecond = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_tx_bytes_per_second",
			Help: "Outgoing network traffic in bytes per second",
		},
		[]string{"interface"},
	)

	NetworkErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "network_errors",
			Help: "Number of network errors",
		},
		[]string{"interface"},
	)

	NetworkDroppedPackets = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "network_dropped_packets",
			Help: "Number of network dropped packets",
		},
		[]string{"interface"},
	)
)

// RecordNetworkMetrics собирает сетевые метрики
func RecordNetworkMetrics() {
	go func() {
		prevStats := make(map[string]net.IOCountersStat)

		for {
			// Получение только физических сетевых адаптеров через WMI
			var networkAdapters []Win32_NetworkAdapter
			err := wmi.Query("SELECT Name, MACAddress, Manufacturer, NetEnabled, PNPDeviceID, Description, NetConnectionID FROM Win32_NetworkAdapter WHERE MACAddress IS NOT NULL AND PhysicalAdapter = TRUE", &networkAdapters)
			if err != nil || len(networkAdapters) == 0 {
				log.Printf("Error getting network adapter info: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// Создание списка только физических интерфейсов
			physicalInterfaces := make(map[string]string)
			for _, adapter := range networkAdapters {
				if adapter.Manufacturer == "Microsoft" ||
					strings.Contains(adapter.PNPDeviceID, "ROOT\\") ||
					strings.Contains(adapter.Description, "Virtual") ||
					strings.Contains(adapter.Name, "WAN Miniport") {
					continue
				}

				physicalInterfaces[adapter.NetConnectionID] = adapter.Name
				status := 0.0
				if adapter.NetEnabled {
					status = 1.0
				}
				NetworkStatus.With(prometheus.Labels{
					"interface": adapter.Name,
				}).Set(status)
			}

			// Получение статистики ввода-вывода для сетевых интерфейсов
			ioStats, err := net.IOCounters(true)
			if err != nil {
				log.Printf("Error getting network stats: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			for _, stat := range ioStats {
				if name, ok := physicalInterfaces[stat.Name]; ok {
					prev, ok := prevStats[stat.Name]
					if ok {
						duration := 5.0
						rxRate := float64(stat.BytesRecv-prev.BytesRecv) / duration
						txRate := float64(stat.BytesSent-prev.BytesSent) / duration

						NetworkRxBytesPerSecond.With(prometheus.Labels{"interface": name}).Set(rxRate)
						NetworkTxBytesPerSecond.With(prometheus.Labels{"interface": name}).Set(txRate)

						NetworkErrors.With(prometheus.Labels{"interface": name}).Add(float64(stat.Errin + stat.Errout))
						NetworkDroppedPackets.With(prometheus.Labels{"interface": name}).Add(float64(stat.Dropin + stat.Dropout))

					}

					prevStats[stat.Name] = stat
				}
			}

			time.Sleep(5 * time.Second)
		}
	}()
}
