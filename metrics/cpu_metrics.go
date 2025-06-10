package metrics

import (
	"crypto/sha256"
	"encoding/binary"
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
	Name                      string
	LoadPercentage            uint32
	NumberOfLogicalProcessors uint32
}

var (
	// Proccessor Load
	CpuUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_usage_percent",
			Help: "Current CPU usage in percent",
		},
		[]string{"core", "processor", "logical_cores"},
	)

	// CPU Temperature
	CpuTemperature = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_temperature",
			Help: "Current CPU temperature in celcius",
		},
		[]string{"sensor"},
	)

	// Processor Hash
	ProcessorHash = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "processor_hash",
			Help: "Hash of the processor",
		},
		[]string{"core", "processor", "logical_cores"},
	)
)

func hashStringToFloat64(s string) float64 {
	h := sha256.Sum256([]byte(s))
	hashNum := binary.LittleEndian.Uint64(h[:8])
	return float64(hashNum)
}

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
					hashValue := hashStringToFloat64(processor.Name)
					ProcessorHash.With(prometheus.Labels{
						"core": fmt.Sprintf("core_%d", i),
					}).Set(hashValue)

					CpuUsage.With(prometheus.Labels{
						"core":          fmt.Sprintf("core_%d", i),
						"processor":     processor.Name,
						"logical_cores": fmt.Sprintf("%d", processor.NumberOfLogicalProcessors),
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
