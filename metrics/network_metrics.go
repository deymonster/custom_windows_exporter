package metrics

import (
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/net"
)

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
			interfaces, err := net.Interfaces()
			if err != nil {
				log.Printf("Error getting network interfaces: %v", err)
				return
			}

			for _, iface := range interfaces {
				status := 0.0

				// Проверка статуса интерфейса через флаги gopsutil
				isUp := false
				for _, flag := range iface.Flags {
					if flag == "up" {
						isUp = true
						break
					}
				}
				if isUp {
					status = 1.0
				}

				NetworkStatus.With(prometheus.Labels{
					"interface": iface.Name,
				}).Set(status)
			}

			ioStats, err := net.IOCounters(true)
			if err != nil {
				log.Printf("Error getting network stats: %v", err)
				return
			}

			for _, stat := range ioStats {
				prev, ok := prevStats[stat.Name]

				if ok {
					duration := 5.0
					rxRate := float64(stat.BytesRecv-prev.BytesRecv) / duration
					txRate := float64(stat.BytesSent-prev.BytesSent) / duration

					NetworkRxBytesPerSecond.With(prometheus.Labels{"interface": stat.Name}).Set(rxRate)
					NetworkTxBytesPerSecond.With(prometheus.Labels{"interface": stat.Name}).Set(txRate)

					NetworkErrors.With(prometheus.Labels{"interface": stat.Name}).Add(float64(stat.Errin + stat.Errout))
					NetworkDroppedPackets.With(prometheus.Labels{"interface": stat.Name}).Add(float64(stat.Dropin + stat.Dropout))
				}

				prevStats[stat.Name] = stat
			}
			time.Sleep(5 * time.Second)
		}
	}()
}
