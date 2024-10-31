package metrics

import (
	"log"

	"github.com/StackExchange/wmi"
	"github.com/prometheus/client_golang/prometheus"
)

type Win32_VideoController struct {
	Name       string
	AdapterRAM uint64
}

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

func RecordGpuInfo() {
	var videoControllers []Win32_VideoController
	err := wmi.Query("SELECT Name, AdapterRAM FROM Win32_VideoController", &videoControllers)
	if err != nil {
		log.Printf("Error getting gpu info: %v", err)
		return
	}

	for _, gpu := range videoControllers {
		GpuInfo.With(prometheus.Labels{
			"name": gpu.Name,
		}).Set(1)
		GpuMemory.With(prometheus.Labels{
			"name": gpu.Name,
		}).Set(float64(gpu.AdapterRAM))
	}
}
