package metrics

import (
	"fmt"
	"log"
	"time"

	"github.com/StackExchange/wmi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/mem"
)

type Win32_PhysicalMemory struct {
	Capacity     uint64
	Manufacturer string
	PartNumber   string
	SerialNumber string
	Speed        uint32
}

var (

	// Memory
	MemoryModuleInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "memory_module_info",
			Help: "Memory module information",
		},
		[]string{"capacity", "manufacturer", "part_number", "serial_number", "speed"},
	)

	TotalMemory = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "total_memory_bytes",
			Help: "Total memory on system",
		},
	)

	UsedMemory = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "used_memory_bytes",
			Help: "Used memory on system",
		},
	)

	FreeMemory = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "free_memory_bytes",
			Help: "Free memory on system",
		},
	)
)

func RecordMemoryModuleInfo() {
	var memModules []Win32_PhysicalMemory
	err := wmi.Query("SELECT Capacity, Manufacturer, PartNumber, SerialNumber, Speed FROM Win32_PhysicalMemory", &memModules)

	if err != nil {
		log.Printf("Error getting memory info: %v", err)
		return
	}

	for _, module := range memModules {
		memoryInGb := float64(module.Capacity) / (1024 * 1024 * 1024)

		MemoryModuleInfo.With(prometheus.Labels{
			"capacity":      fmt.Sprintf("%.2fGb", memoryInGb),
			"manufacturer":  module.Manufacturer,
			"part_number":   module.PartNumber,
			"serial_number": module.SerialNumber,
			"speed":         fmt.Sprintf("%dMhz", module.Speed),
		}).Set(memoryInGb)
	}
}

func RecordMemoryUsage() {
	go func() {
		for {
			v, err := mem.VirtualMemory()
			if err != nil {
				log.Printf("Error getting memory info: %v", err)
				return
			}

			TotalMemory.Set(float64(v.Total))
			UsedMemory.Set(float64(v.Used))
			FreeMemory.Set(float64(v.Available))

			time.Sleep(5 * time.Second)
		}
	}()

}
