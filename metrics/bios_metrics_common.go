package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	BiosInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "bios_info",
			Help: "BIOS information",
		},
		[]string{"manufacturer", "version", "release_date"},
	)
)
