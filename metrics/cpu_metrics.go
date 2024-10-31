package metrics

import (
	"fmt"
	"log"
	"time"

	"github.com/StackExchange/wmi"
	"github.com/prometheus/client_golang/prometheus"
)

type Win32_ThermalZoneInformation struct {
	Name        string
	Temperature uint32
}

type Win32_Processor struct {
	Name           string
	LoadPercentage uint32
}

var (
	// Proccessor Load
	CpuUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_usage_percent",
			Help: "Current CPU usage in percent",
		},
		[]string{"core", "processor"},
	)

	// CPU Temperature
	CpuTemperature = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_temperature",
			Help: "Current CPU temperature in celcius",
		},
		[]string{"sensor"},
	)
)

func RecordCPUInfo() {
	go func() {
		for {
			// cpu usage
			var processors []Win32_Processor
			err := wmi.Query("SELECT * FROM Win32_Processor", &processors)

			if err != nil {
				log.Printf("Error getting cpu info: %v", err)
			} else {
				for i, processor := range processors {
					CpuUsage.With(prometheus.Labels{
						"core":      fmt.Sprintf("core_%d", i),
						"processor": processor.Name,
					}).Set(float64(processor.LoadPercentage))
				}
			}

			// cpu temperature
			var temps []Win32_ThermalZoneInformation
			err = wmi.Query("SELECT Name, Temperature FROM Win32_PerfRawData_Counters_ThermalZoneInformation", &temps)
			if err != nil {
				log.Printf("Error getting cpu info: %v", err)
			} else {
				for _, temp := range temps {
					tempC := float64(temp.Temperature) - 273.15
					CpuTemperature.With(prometheus.Labels{
						"sensor": temp.Name,
					}).Set(tempC)
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()
}
