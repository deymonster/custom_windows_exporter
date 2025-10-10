package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	DiskUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_usage_bytes",
			Help: "Disk usage on system",
		},
		[]string{"disk", "model", "serial", "type"},
	)

	DiskUsagePercent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_usage_percent",
			Help: "Disk usage on system",
		},
		[]string{"disk", "model", "serial"},
	)

	DiskReadBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_read_bytes_per_second",
			Help: "Disk read bytes per second",
		},
		[]string{"disk", "model", "serial"},
	)

	DiskWriteBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_write_bytes_per_second",
			Help: "Disk write bytes per second",
		},
		[]string{"disk", "model", "serial"},
	)

	DiskHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_health_status",
			Help: "Health status of disk",
		},
		[]string{"disk", "serial", "type", "status", "size"},
	)
)
