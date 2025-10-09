//go:build linux

package metrics

import (
	"fmt"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/mem"
)

func RecordMemoryModuleInfo() {
	stats, err := mem.VirtualMemory()
	if err != nil {
		log.Printf("failed to read memory stats: %v", err)
		return
	}

	capacityGB := float64(stats.Total) / (1024 * 1024 * 1024)
	MemoryModuleInfo.With(prometheus.Labels{
		"capacity":      fmt.Sprintf("%.2fGb", capacityGB),
		"manufacturer":  "unknown",
		"part_number":   "unknown",
		"serial_number": "unknown",
		"speed":         "unknown",
	}).Set(capacityGB)
}

func RecordMemoryUsage() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			stats, err := mem.VirtualMemory()
			if err != nil {
				log.Printf("failed to read virtual memory stats: %v", err)
				<-ticker.C
				continue
			}

			TotalMemory.Set(float64(stats.Total))
			UsedMemory.Set(float64(stats.Used))
			FreeMemory.Set(float64(stats.Available))

			<-ticker.C
		}
	}()
}
