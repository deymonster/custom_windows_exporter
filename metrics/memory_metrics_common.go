package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
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
