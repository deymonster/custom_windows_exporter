package metrics

import (
	"log"

	"node_exporter_custom/internal/mockconfig"
)

func ApplyMockConfig(cfg mockconfig.Config) {
	if cfg.UUIDChanged {
		HardwareUUIDChanged.Set(1)
	} else {
		HardwareUUIDChanged.Set(0)
	}

	CpuUsage.WithLabelValues("core_0", "Mock CPU", "4").Set(float64(cfg.CPULoad))

	CpuTemperature.WithLabelValues("Mock Sensor").Set(cfg.CPUTemperature)

	totalSpace := float64(1000000000000)
	freeSpace := totalSpace * float64(cfg.DiskFreePercent) / 100.0
	usedSpace := totalSpace - freeSpace
	DiskUsage.WithLabelValues("C:", "Mock Disk", "SSD").Set(usedSpace)
	DiskUsagePercent.WithLabelValues("C:", "Mock Disk").Set(100.0 - float64(cfg.DiskFreePercent))

	DiskReadBytes.WithLabelValues("C:", "Mock Disk").Set(float64(cfg.DiskReadBytesPerSec))
	DiskWriteBytes.WithLabelValues("C:", "Mock Disk").Set(float64(cfg.DiskWriteBytesPerSec))

	totalMemory := float64(16000000000)
	freeMemory := totalMemory * float64(cfg.MemoryFreePercent) / 100.0
	usedMemory := totalMemory - freeMemory

	TotalMemory.Set(totalMemory)
	FreeMemory.Set(freeMemory)
	UsedMemory.Set(usedMemory)

	NetworkErrors.WithLabelValues("Mock Ethernet").Add(float64(cfg.NetworkErrors))
}

func LogMockEnabled() {
	log.Println("Mock metrics enabled")
}
