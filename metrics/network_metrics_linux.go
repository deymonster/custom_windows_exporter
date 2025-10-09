//go:build linux

package metrics

import (
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/net"
)

func RecordNetworkMetrics() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		prevStats := make(map[string]net.IOCountersStat)

		for {
			interfaces, err := net.Interfaces()
			if err != nil {
				log.Printf("failed to list network interfaces: %v", err)
				<-ticker.C
				continue
			}

			active := make(map[string]struct{})
			for _, iface := range interfaces {
				if iface.Name == "" {
					continue
				}

				status := 0.0
				for _, flag := range iface.Flags {
					if flag == "up" {
						status = 1.0
						break
					}
				}
				if status > 0 {
					active[iface.Name] = struct{}{}
				}
				NetworkStatus.With(prometheus.Labels{"interface": iface.Name}).Set(status)
			}

			stats, err := net.IOCounters(true)
			if err != nil {
				log.Printf("failed to read network counters: %v", err)
				<-ticker.C
				continue
			}

			for _, stat := range stats {
				if _, ok := active[stat.Name]; !ok {
					// still record rates for interfaces even if flagged down
					NetworkStatus.With(prometheus.Labels{"interface": stat.Name}).Set(0)
				}

				if prev, ok := prevStats[stat.Name]; ok {
					duration := 5.0
					rxRate := float64(stat.BytesRecv-prev.BytesRecv) / duration
					txRate := float64(stat.BytesSent-prev.BytesSent) / duration

					if rxRate < 0 {
						rxRate = 0
					}
					if txRate < 0 {
						txRate = 0
					}

					NetworkRxBytesPerSecond.With(prometheus.Labels{"interface": stat.Name}).Set(rxRate)
					NetworkTxBytesPerSecond.With(prometheus.Labels{"interface": stat.Name}).Set(txRate)

					errDelta := (stat.Errin + stat.Errout) - (prev.Errin + prev.Errout)
					dropDelta := (stat.Dropin + stat.Dropout) - (prev.Dropin + prev.Dropout)
					if errDelta < 0 {
						errDelta = 0
					}
					if dropDelta < 0 {
						dropDelta = 0
					}
					if errDelta > 0 {
						NetworkErrors.With(prometheus.Labels{"interface": stat.Name}).Add(float64(errDelta))
					}
					if dropDelta > 0 {
						NetworkDroppedPackets.With(prometheus.Labels{"interface": stat.Name}).Add(float64(dropDelta))
					}
				}

				prevStats[stat.Name] = stat
			}

			<-ticker.C
		}
	}()
}
