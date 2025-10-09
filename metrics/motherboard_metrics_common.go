package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	MotherboardInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "motherboard_info",
			Help: "Motherboard info",
		},
		[]string{"manufacturer", "product", "serial_number", "version"},
	)
)
