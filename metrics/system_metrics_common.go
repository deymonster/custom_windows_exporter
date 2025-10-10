package metrics

import "github.com/prometheus/client_golang/prometheus"

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
