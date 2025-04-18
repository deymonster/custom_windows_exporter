package metrics

import (
	"fmt"
	"log"
	"time"

	"github.com/StackExchange/wmi"
	"github.com/prometheus/client_golang/prometheus"
)

type Win32_ComputerSystem struct {
	Name         string
	Manufacturer string
	Model        string
}

type Win32_OperatingSystem struct {
	Caption        string
	Version        string
	OSArchitecture string
	LastBootUpTime time.Time
}

var (
	SystemInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_information",
			Help: "Basic system information: name, OS version, OS architecture, manufacturer, model",
		},
		[]string{"name", "os_version", "os_architecture", "manufacturer", "model"},
	)

	SystemUptime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "system_uptime",
			Help: "System uptime in seconds",
		},
	)
)

func RecordSystemMetrics() {
	go func() {
		var computerSystem []Win32_ComputerSystem
		var operatingSystem []Win32_OperatingSystem

		err := wmi.Query("SELECT Name, Manufacturer, Model FROM Win32_ComputerSystem", &computerSystem)
		if err != nil || len(computerSystem) == 0 {
			log.Printf("Error getting computer system info: %v", err)
			return
		}

		err = wmi.Query("SELECT Caption, Version, OSArchitecture, LastBootUpTime FROM Win32_OperatingSystem", &operatingSystem)
		if err != nil || len(operatingSystem) == 0 {
			log.Printf("Error getting operating system info: %v", err)
			return
		}

		cs := computerSystem[0]
		os := operatingSystem[0]

		uptime := time.Since(os.LastBootUpTime).Seconds()

		SystemInfo.With(prometheus.Labels{
			"name":            cs.Name,
			"os_version":      fmt.Sprintf("%s %s", os.Caption, os.Version),
			"os_architecture": os.OSArchitecture,
			"manufacturer":    cs.Manufacturer,
			"model":           cs.Model,
		}).Set(1)

		SystemUptime.Set(uptime)

		time.Sleep(5 * time.Second)
	}()
}
