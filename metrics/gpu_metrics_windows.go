// импорт, структура
//go:build windows

package metrics

import (
    "fmt"
    "log"
    "strings"

    "github.com/StackExchange/wmi"
    "github.com/prometheus/client_golang/prometheus"
)

type Win32_VideoController struct {
    Name                 string
    AdapterRAM           uint64
    PNPDeviceID          string
    AdapterCompatibility string
}

// GetGPUInfo retrieves information about video controllers (GPUs) in the system
// by querying the Win32_VideoController WMI class. It returns a slice of
// Win32_VideoController structs containing details such as the name and
// adapter RAM of each video controller, or an error if the query fails.

// запрос WMI
func GetGPUInfo() ([]Win32_VideoController, error) {
    var videoControllers []Win32_VideoController
    err := wmi.Query("SELECT Name, AdapterRAM, PNPDeviceID, AdapterCompatibility FROM Win32_VideoController", &videoControllers)
    if err != nil {
        return nil, fmt.Errorf("error getting GPU info: %v", err)
    }
    return videoControllers, nil
}

// RecordGpuInfo records information about GPUs in the system to Prometheus.
// It is designed to be run as a goroutine in a loop.
// запись метрик
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
        GpuType.With(prometheus.Labels{
            "name": gpu.Name,
            "type": classifyGPUTypeWindows(gpu),
        }).Set(1)
    }
}

// классификация для Windows
func classifyGPUTypeWindows(vc Win32_VideoController) string {
    ven := extractVendorID(vc.PNPDeviceID)
    name := strings.ToLower(vc.Name)
    compat := strings.ToLower(vc.AdapterCompatibility)
    v := strings.ToLower(ven)

    if v == "8086" || strings.Contains(name, "intel") || strings.Contains(compat, "intel") {
        return "integrated"
    }
    if v == "10de" || strings.Contains(name, "nvidia") || strings.Contains(compat, "nvidia") {
        return "discrete"
    }
    if v == "1002" || strings.Contains(name, "amd") || strings.Contains(name, "radeon") || strings.Contains(compat, "amd") {
        if strings.Contains(name, "vega") || strings.Contains(name, "ryzen") || strings.Contains(name, "apu") || strings.Contains(name, "radeon graphics") {
            return "integrated"
        }
        return "discrete"
    }

    return "unknown"
}

func extractVendorID(pnp string) string {
    p := strings.ToUpper(pnp)
    idx := strings.Index(p, "VEN_")
    if idx >= 0 && len(p) >= idx+8 {
        return p[idx+4 : idx+8]
    }
    return ""
}
