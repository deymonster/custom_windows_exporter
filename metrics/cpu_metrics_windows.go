//go:build windows

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
	Name                      string
	LoadPercentage            uint32
	NumberOfLogicalProcessors uint32
}

// getCpuInfo retrieves information about all processors in the system
// by querying the Win32_Processor WMI class. It returns a slice of
// Win32_Processor structs containing details such as processor name,
// load percentage, and number of logical processors.

func GetCPUInfo() ([]Win32_Processor, error) {
	var processors []Win32_Processor
	err := wmi.Query("SELECT * FROM Win32_Processor", &processors)
	if err != nil {
		return nil, fmt.Errorf("error getting cpu info: %v", err)
	}
	return processors, nil
}

// GetCPUTemperature retrieves temperature information for the CPU
// by querying the Win32_PerfRawData_Counters_ThermalZoneInformation WMI class.
// It returns a slice of Win32_ThermalZoneInformation structs containing
// the names and temperatures of the thermal zones, or an error if the query fails.

func GetCPUTemperature() ([]Win32_ThermalZoneInformation, error) {
	var temps []Win32_ThermalZoneInformation
	err := wmi.Query("SELECT Name, Temperature FROM Win32_PerfRawData_Counters_ThermalZoneInformation", &temps)
	if err != nil {
		return nil, fmt.Errorf("error getting cpu temperature: %v", err)
	}
	return temps, nil
}

// RecordCPUInfo continuously collects and records CPU metrics including
// CPU usage and temperature. It retrieves processor load percentages
// and thermal zone temperatures using WMI queries. The collected data
// is then recorded using Prometheus metrics. This function runs
// indefinitely, updating metrics every 5 seconds.

func RecordCPUInfo() {
	go func() {
		for {
			// Получаем и записываем информацию о загрузке процессора
			processors, err := GetCPUInfo()
			if err != nil {
				log.Printf("%v", err)
			} else {
				for i, processor := range processors {
					CpuUsage.With(prometheus.Labels{
						"core":          fmt.Sprintf("core_%d", i),
						"processor":     processor.Name,
						"logical_cores": fmt.Sprintf("%d", processor.NumberOfLogicalProcessors),
					}).Set(float64(processor.LoadPercentage))
				}
			}

			// Получаем и записываем информацию о температуре
			temps, err := GetCPUTemperature()
			if err != nil {
				log.Printf("%v", err)
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
