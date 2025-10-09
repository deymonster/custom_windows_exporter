package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	SystemUUID = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "UNIQUE_ID_SYSTEM",
			Help: "Unique ID for the system",
		},
		[]string{"uuid"},
	)

	HardwareUUIDChanged = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "UNIQUE_ID_CHANGED",
			Help: "Indicates if the system UUID has changed",
		},
	)
)
