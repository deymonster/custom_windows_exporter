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

		manufacturer := info.Vendor
		if manufacturer == "" {
			manufacturer = info.HostID
		}

		SystemInfo.With(prometheus.Labels{
			"name":            info.Hostname,
			"os_version":      fmt.Sprintf("%s %s", info.Platform, info.PlatformVersion),
			"os_architecture": info.KernelArch,
			"manufacturer":    manufacturer,
			"model":           info.KernelVersion,
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
