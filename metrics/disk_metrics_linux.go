//go:build linux

package metrics

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/disk"
)

func RecordDiskUsage() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		prevIO := make(map[string]disk.IOCountersStat)

		for {
			partitions, err := disk.Partitions(false)
			if err != nil {
				log.Printf("failed to list disk partitions: %v", err)
				<-ticker.C
				continue
			}

			ioCounters, err := disk.IOCounters()
			if err != nil {
				log.Printf("failed to read disk IO counters: %v", err)
				ioCounters = map[string]disk.IOCountersStat{}
			}

			for _, part := range partitions {
				if part.Mountpoint == "" {
					continue
				}

				usage, err := disk.Usage(part.Mountpoint)
				if err != nil {
					log.Printf("failed to read usage for %s: %v", part.Mountpoint, err)
					continue
				}

				model := part.Device
				diskLabel := part.Device
				if diskLabel == "" {
					diskLabel = part.Mountpoint
				}

				DiskUsage.With(prometheus.Labels{
					"disk":  diskLabel,
					"model": model,
					"type":  "total",
				}).Set(float64(usage.Total))

				DiskUsage.With(prometheus.Labels{
					"disk":  diskLabel,
					"model": model,
					"type":  "free",
				}).Set(float64(usage.Free))

				DiskUsage.With(prometheus.Labels{
					"disk":  diskLabel,
					"model": model,
					"type":  "used",
				}).Set(float64(usage.Used))

				usedPercent := 0.0
				if usage.Total > 0 {
					usedPercent = (float64(usage.Used) / float64(usage.Total)) * 100
				}

				DiskUsagePercent.With(prometheus.Labels{
					"disk":  diskLabel,
					"model": model,
				}).Set(usedPercent)

				baseName := strings.TrimPrefix(filepath.Base(part.Device), "/dev/")
				if baseName == "" {
					baseName = filepath.Base(part.Device)
				}
				if baseName == "" {
					baseName = diskLabel
				}

				if counter, ok := ioCounters[baseName]; ok {
					if prev, exists := prevIO[baseName]; exists {
						duration := 5.0
						readRate := float64(counter.ReadBytes-prev.ReadBytes) / duration
						writeRate := float64(counter.WriteBytes-prev.WriteBytes) / duration

						DiskReadBytes.With(prometheus.Labels{
							"disk":  diskLabel,
							"model": model,
						}).Set(readRate)

						DiskWriteBytes.With(prometheus.Labels{
							"disk":  diskLabel,
							"model": model,
						}).Set(writeRate)
					}
					prevIO[baseName] = counter
				}

				DiskHealthStatus.With(prometheus.Labels{
					"disk":   diskLabel,
					"type":   part.Fstype,
					"status": "unknown",
					"size":   fmt.Sprintf("%d", usage.Total),
				}).Set(1)
			}

			<-ticker.C
		}
	}()
}
