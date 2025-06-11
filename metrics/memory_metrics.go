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


// GetMemoryModules retrieves information about physical memory modules in the system
// by querying the Win32_PhysicalMemory WMI class. It returns a slice of
// Win32_PhysicalMemory structs containing details such as capacity, manufacturer,
// part number, serial number, and speed, or an error if the query fails.

func GetMemoryModules() ([]Win32_PhysicalMemory, error) {
	var memModules []Win32_PhysicalMemory
	err := wmi.Query("SELECT Capacity, Manufacturer, PartNumber, SerialNumber, Speed FROM Win32_PhysicalMemory", &memModules)
	if err != nil {
		return nil, fmt.Errorf("error getting memory modules info: %v", err)
	}
	return memModules, nil
}


// GetMemoryUsage retrieves information about virtual memory usage on the system.
// It returns a mem.VirtualMemoryStat struct containing details such as total,
// used, free, and cached memory, or an error if the query fails.
func GetMemoryUsage() (*mem.VirtualMemoryStat, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("error getting memory usage: %v", err)
	}
	return v, nil
}


// RecordMemoryModuleInfo records information about physical memory modules in the system
// to prometheus metrics. It records the capacity in GB, manufacturer, part number, serial
// number, and speed of each module. It runs in a separate goroutine and updates the
// metrics every 5 seconds.
func RecordMemoryModuleInfo() {
	modules, err := GetMemoryModules()

	if err != nil {
		log.Printf("Error getting memory info: %v", err)
		return
	}

	for _, module := range modules {
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
// RecordMemoryUsage records virtual memory usage on the system in prometheus metrics.
// It records total, used, and free memory in bytes. It runs in a separate goroutine
// and updates the metrics every 5 seconds.

func RecordMemoryUsage() {
	go func() {
		for {
			memStat, err := GetMemoryUsage()
			if err != nil {
				log.Printf("Error getting memory info: %v", err)
				return
			}

			TotalMemory.Set(float64(memStat.Total))
			UsedMemory.Set(float64(memStat.Used))
			FreeMemory.Set(float64(memStat.Available))

			time.Sleep(5 * time.Second)
		}
	}()

}
