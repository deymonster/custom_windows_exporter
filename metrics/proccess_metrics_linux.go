//go:build linux

package metrics

import (
	"fmt"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/process"
)

type processTotals struct {
	count      int
	rssMB      float64
	privateMB  float64
	cpuPercent float64
}

func RecordProccessInfo() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			processes, err := process.Processes()
			if err != nil {
				log.Printf("failed to enumerate processes: %v", err)
				<-ticker.C
				continue
			}

			ProccessCount.Set(float64(len(processes)))

			aggregates := make(map[string]*processTotals)

			for _, proc := range processes {
				pid := proc.Pid
				name, err := proc.Name()
				if err != nil || name == "" {
					name = fmt.Sprintf("pid_%d", pid)
				}

				labels := prometheus.Labels{"process": name, "pid": fmt.Sprintf("%d", pid)}

				if memInfo, err := proc.MemoryInfo(); err == nil {
					rssMB := float64(memInfo.RSS) / (1024 * 1024)
					privMB := float64(memInfo.VMS) / (1024 * 1024)

					ProccessMemoryUsage.With(labels).Set(rssMB)

					agg := aggregates[name]
					if agg == nil {
						agg = &processTotals{}
						aggregates[name] = agg
					}
					agg.count++
					agg.rssMB += rssMB
					agg.privateMB += privMB
				}

				if cpuPercent, err := proc.CPUPercent(); err == nil {
					ProccessCPUUsage.With(labels).Set(cpuPercent)

					agg := aggregates[name]
					if agg == nil {
						agg = &processTotals{}
						aggregates[name] = agg
					}
					if agg.count == 0 {
						agg.count = 1
					}
					agg.cpuPercent += cpuPercent
				}
			}

			for name, totals := range aggregates {
				ProcessInstanceCount.With(prometheus.Labels{"process": name}).Set(float64(totals.count))
				ProcessGroupMemoryWorkingSet.With(prometheus.Labels{
					"process":   name,
					"instances": fmt.Sprintf("%d", totals.count),
				}).Set(totals.rssMB)
				ProcessGroupMemoryPrivate.With(prometheus.Labels{
					"process":   name,
					"instances": fmt.Sprintf("%d", totals.count),
				}).Set(totals.privateMB)
				ProcessGroupCPUUsage.With(prometheus.Labels{
					"process":   name,
					"instances": fmt.Sprintf("%d", totals.count),
				}).Set(totals.cpuPercent)
			}

			<-ticker.C
		}
	}()
}
