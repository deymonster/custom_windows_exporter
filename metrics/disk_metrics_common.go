package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	DiskUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_usage_bytes",
			Help: "Disk usage on system",
		},
		[]string{"disk", "model", "type"},
	)

	DiskUsagePercent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_usage_percent",
			Help: "Disk usage on system",
		},
		[]string{"disk", "model"},
	)

	DiskReadBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_read_bytes_per_second",
			Help: "Disk read bytes per second",
		},
		[]string{"disk", "model"},
	)

	DiskWriteBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_write_bytes_per_second",
			Help: "Disk write bytes per second",
		},
		[]string{"disk", "model"},
	)

	DiskHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_health_status",
			Help: "Health status of disk",
		},
		[]string{"disk", "type", "status", "size"},
	)
)
