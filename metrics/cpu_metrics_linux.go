//go:build linux

package metrics

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
    "strings"
)

func RecordCPUInfo() {
	go func() {
		// warm-up call so subsequent percent calculations have delta
		if _, err := cpu.Percent(0, true); err != nil {
			log.Printf("failed to initialize cpu percent collection: %v", err)
		}

		cpuInfo, err := cpu.Info()
		if err != nil {
			log.Printf("failed to query cpu info: %v", err)
		}

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			percentages, err := cpu.Percent(0, true)
			if err != nil {
				log.Printf("failed to collect cpu percent: %v", err)
			} else {
				logicalCores := fmt.Sprintf("%d", runtime.NumCPU())
				for idx, pct := range percentages {
					model := "unknown"
					if idx < len(cpuInfo) {
						model = cpuInfo[idx].ModelName
					} else if len(cpuInfo) > 0 {
						model = cpuInfo[0].ModelName
					}

					CpuUsage.With(prometheus.Labels{
						"core":          fmt.Sprintf("core_%d", idx),
						"processor":     model,
						"logical_cores": logicalCores,
					}).Set(pct)
				}
			}

			temps, err := host.SensorsTemperatures()
			if err != nil {
				log.Printf("failed to collect cpu temperatures: %v", err)
			} else {
				for _, sensor := range temps {
					if sensor.Temperature == 0 {
						continue
					}
					label, ok := normalizeTempSensorLabel(sensor.SensorKey)
					if !ok {
						continue
					}
					CpuTemperature.With(prometheus.Labels{
						"sensor": label,
					}).Set(sensor.Temperature)
				}
			}

			<-ticker.C
		}
	}()
}

// helper: normalizeTempSensorLabel
func normalizeTempSensorLabel(key string) (string, bool) {
    k := strings.ToLower(strings.TrimSpace(key))
    // отсекаем нерабочие метрики
    if strings.Contains(k, "_crit") || strings.Contains(k, "_max") {
        return "", false
    }
    // принимаем только *_input (рабочее значение)
    if strings.HasSuffix(k, "_input") {
        k = strings.TrimSuffix(k, "_input")
    } else {
        return "", false
    }

    switch {
    case strings.HasPrefix(k, "coretemp_core"):
        idx := strings.TrimPrefix(k, "coretemp_core")
        return "core_" + idx, true
    case strings.HasPrefix(k, "coretemp_packageid"):
        pid := strings.TrimPrefix(k, "coretemp_packageid")
        return "package_" + pid, true
    case strings.HasPrefix(k, "acpitz"):
        return "acpi_tz", true
    case strings.HasPrefix(k, "pch_"):
        return "pch", true
    default:
        // оставим нормализованный ключ как есть
        return k, true
    }
}
