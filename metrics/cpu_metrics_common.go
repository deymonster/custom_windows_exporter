package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	CpuUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_usage_percent",
			Help: "Current CPU usage in percent",
		},
		[]string{"core", "processor", "logical_cores"},
	)

	CpuTemperature = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cpu_temperature",
			Help: "Current CPU temperature in celcius",
		},
		[]string{"sensor"},
	)
)
