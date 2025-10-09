//go:build linux

package metrics

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
)

func RecordCPUInfo() {
	go func() {
		// warm-up call so subsequent percent calculations have delta
		if _, err := cpu.Percent(0, true); err != nil {
			log.Printf("failed to initialize cpu percent collection: %v", err)
		}

		cpuInfo, err := cpu.Info()
		if err != nil {
			log.Printf("failed to query cpu info: %v", err)
		}

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			percentages, err := cpu.Percent(0, true)
			if err != nil {
				log.Printf("failed to collect cpu percent: %v", err)
			} else {
				logicalCores := fmt.Sprintf("%d", runtime.NumCPU())
				for idx, pct := range percentages {
					model := "unknown"
					if idx < len(cpuInfo) {
						model = cpuInfo[idx].ModelName
					} else if len(cpuInfo) > 0 {
						model = cpuInfo[0].ModelName
					}

					CpuUsage.With(prometheus.Labels{
						"core":          fmt.Sprintf("core_%d", idx),
						"processor":     model,
						"logical_cores": logicalCores,
					}).Set(pct)
				}
			}

			temps, err := host.SensorsTemperatures()
			if err != nil {
				log.Printf("failed to collect cpu temperatures: %v", err)
			} else {
				for _, sensor := range temps {
					if sensor.Temperature == 0 {
						continue
					}
					CpuTemperature.With(prometheus.Labels{
						"sensor": sensor.SensorKey,
					}).Set(sensor.Temperature)
				}
			}

			<-ticker.C
		}
	}()
}
