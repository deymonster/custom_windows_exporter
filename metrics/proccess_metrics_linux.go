//go:build linux

package metrics

import (
	"fmt"
	"log"
	"strings"
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
				name := processName(proc, pid)

				labels := prometheus.Labels{"process": name, "pid": fmt.Sprintf("%d", pid)}

				if memInfo, err := proc.MemoryInfoEx(); err == nil {
					rssMB := float64(memInfo.RSS) / (1024 * 1024)
					privateBytes := memInfo.RSS
					if memInfo.Shared < memInfo.RSS {
						privateBytes = memInfo.RSS - memInfo.Shared
					}
					privMB := float64(privateBytes) / (1024 * 1024)

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

func processName(proc *process.Process, pid int32) string {
	name, err := proc.Name()
	if err != nil {
		name = ""
	}

	if sanitized := sanitizeProcessLabel(name); sanitized != "" {
		return sanitized
	}

	if cmdline, err := proc.Cmdline(); err == nil {
		if sanitized := sanitizeProcessLabel(cmdline); sanitized != "" {
			return sanitized
		}
	}

	return fmt.Sprintf("pid_%d", pid)
}

func sanitizeProcessLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if idx := strings.LastIndex(raw, "/"); idx >= 0 {
		raw = raw[idx+1:]
	}

	if idx := strings.Index(raw, " "); idx > 0 {
		raw = raw[:idx]
	}

	raw = strings.TrimSpace(raw)
	return raw
}
