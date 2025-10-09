//go:build windows

package metrics

import (
	"fmt"
	"log"

	"github.com/StackExchange/wmi"
	"github.com/prometheus/client_golang/prometheus"
)

type Win32_VideoController struct {
	Name       string
	AdapterRAM uint64
}

// GetGPUInfo retrieves information about video controllers (GPUs) in the system
// by querying the Win32_VideoController WMI class. It returns a slice of
// Win32_VideoController structs containing details such as the name and
// adapter RAM of each video controller, or an error if the query fails.

func GetGPUInfo() ([]Win32_VideoController, error) {
	var videoControllers []Win32_VideoController
	err := wmi.Query("SELECT Name, AdapterRAM FROM Win32_VideoController", &videoControllers)
	if err != nil {
		return nil, fmt.Errorf("error getting GPU info: %v", err)
	}
	return videoControllers, nil
}

// RecordGpuInfo records information about GPUs in the system to Prometheus.
// It is designed to be run as a goroutine in a loop.
func RecordGpuInfo() {
	videoControllers, err := GetGPUInfo()
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
