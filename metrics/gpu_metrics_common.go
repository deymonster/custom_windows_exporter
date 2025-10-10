package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	GpuInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gpu_info",
			Help: "GPU info on system",
		},
		[]string{"name"},
	)

	GpuMemory = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gpu_memory_bytes",
			Help: "GPU memory on system in bytes",
		},
		[]string{"name"},
	)
)
