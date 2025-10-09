//go:build linux

package metrics

import (
	"fmt"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/host"
)

func RecordSystemMetrics() {
	go func() {
		info, err := host.Info()
		if err != nil {
			log.Printf("failed to read host info: %v", err)
			return
		}

		manufacturer := readSysfsValue("/sys/class/dmi/id/sys_vendor")
		if manufacturer == "" {
			manufacturer = info.Platform
		}

		model := readSysfsValue("/sys/class/dmi/id/product_name")
		if model == "" {
			model = info.KernelVersion
		}

		SystemInfo.With(prometheus.Labels{
			"name":            info.Hostname,
			"os_version":      fmt.Sprintf("%s %s", info.Platform, info.PlatformVersion),
			"os_architecture": info.KernelArch,
			"manufacturer":    manufacturer,
			"model":           model,
		}).Set(1)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			uptime, err := host.Uptime()
			if err != nil {
				log.Printf("failed to read uptime: %v", err)
			} else {
				SystemUptime.Set(float64(uptime))
			}

			<-ticker.C
		}
	}()
}
