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
	"unicode"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/disk"
)

type diskMetadata struct {
	Model  string
	Serial string
}

type diskHealthEntry struct {
	Status    string
	CheckedAt time.Time
}

var (
	diskHealthCache = map[string]diskHealthEntry{}
	diskHealthMu    sync.Mutex
)

func RecordDiskUsage() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		prevIO := make(map[string]disk.IOCountersStat)

		for {
			metadata := loadDiskMetadata()

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

				baseName := diskBaseName(part.Device)
				if baseName == "" {
					baseName = filepath.Base(part.Device)
				}

				if strings.HasPrefix(baseName, "loop") || strings.HasPrefix(baseName, "ram") || baseName == "" {
					continue
				}

				meta := metadata[baseName]
				model := meta.Model
				if model == "" {
					model = baseName
				}

				diskLabel := part.Mountpoint
				if diskLabel == "" {
					diskLabel = part.Device
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
					"status": diskHealthStatus(baseName),
					"size":   fmt.Sprintf("%d", usage.Total),
				}).Set(1)
			}

			<-ticker.C
		}
	}()
}

func loadDiskMetadata() map[string]diskMetadata {
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return map[string]diskMetadata{}
	}

	metadata := make(map[string]diskMetadata)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") || strings.HasPrefix(name, "fd") {
			continue
		}

		model := readSysfsValue(filepath.Join("/sys/block", name, "device", "model"))
		vendor := readSysfsValue(filepath.Join("/sys/block", name, "device", "vendor"))
		serial := readSysfsValue(filepath.Join("/sys/block", name, "device", "serial"))

		fullModel := strings.TrimSpace(strings.Join([]string{vendor, model}, " "))
		if fullModel == "" {
			fullModel = name
		}

		metadata[name] = diskMetadata{
			Model:  fullModel,
			Serial: serial,
		}
	}

	return metadata
}

func diskBaseName(device string) string {
	base := filepath.Base(device)
	base = strings.TrimPrefix(base, "/dev/")

	switch {
	case strings.HasPrefix(base, "nvme"):
		if idx := strings.Index(base, "p"); idx > 0 {
			return base[:idx]
		}
		return base
	case strings.HasPrefix(base, "mmcblk"):
		if idx := strings.Index(base, "p"); idx > 0 {
			return base[:idx]
		}
		return base
	default:
		if strings.Contains(base, "-") {
			return base
		}
		return strings.TrimRightFunc(base, func(r rune) bool {
			return unicode.IsDigit(r)
		})
	}
}

func diskHealthStatus(base string) string {
	diskHealthMu.Lock()
	entry, ok := diskHealthCache[base]
	diskHealthMu.Unlock()

	if ok && time.Since(entry.CheckedAt) < time.Minute {
		return entry.Status
	}

	status := queryDiskHealth(base)
	if status == "" {
		status = "unknown"
	}

	diskHealthMu.Lock()
	diskHealthCache[base] = diskHealthEntry{Status: status, CheckedAt: time.Now()}
	diskHealthMu.Unlock()

	return status
}

func queryDiskHealth(base string) string {
	if strings.HasPrefix(base, "nvme") {
		controller := base
		if idx := strings.Index(base, "n"); idx > 0 {
			controller = base[:idx]
		}

		if status := runNvmeCLI("/dev/" + controller); status != "" {
			return status
		}

		if status := runSmartctl([]string{"-H", "-d", "nvme", "/dev/" + controller}); status != "" {
			return status
		}
	}

	return runSmartctl([]string{"-H", "/dev/" + base})
}

func runSmartctl(args []string) string {
	cmd := exec.Command("smartctl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	lower := strings.ToLower(string(output))
	switch {
	case strings.Contains(lower, "passed"):
		return "healthy"
	case strings.Contains(lower, "warning"), strings.Contains(lower, "prefail"), strings.Contains(lower, "degrad"):
		return "warning"
	case strings.Contains(lower, "fail"):
		return "unhealthy"
	default:
		return "unknown"
	}
}

func runNvmeCLI(device string) string {
	cmd := exec.Command("nvme", "smart-log", device)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "critical_warning") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				value := fields[len(fields)-1]
				value = strings.TrimPrefix(value, "0x")
				if value == "0" {
					return "healthy"
				}
				return "warning"
			}
		}
	}

	return ""
}
