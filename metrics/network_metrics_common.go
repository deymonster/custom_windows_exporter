package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	NetworkStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_status",
			Help: "Network status on system (1 = up, 0 = down)",
		},
		[]string{"interface"},
	)

	NetworkRxBytesPerSecond = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_rx_bytes_per_second",
			Help: "Incoming network traffic in bytes per second",
		},
		[]string{"interface"},
	)

	NetworkTxBytesPerSecond = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "network_tx_bytes_per_second",
			Help: "Outgoing network traffic in bytes per second",
		},
		[]string{"interface"},
	)

	NetworkErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "network_errors",
			Help: "Number of network errors",
		},
		[]string{"interface"},
	)

	NetworkDroppedPackets = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "network_dropped_packets",
			Help: "Number of network dropped packets",
		},
		[]string{"interface"},
	)
)
