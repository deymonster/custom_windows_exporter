//go:build linux

package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	ServerProfileInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "nitrino_server_profile_info", Help: "Enabled monitoring profile."},
		[]string{"profile"},
	)
	ServerDMIInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "nitrino_server_dmi_info", Help: "Server and baseboard inventory from DMI."},
		[]string{"system_vendor", "system_model", "board_vendor", "board_model", "board_serial"},
	)
	ServerIPMIInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "nitrino_server_ipmi_info", Help: "Local BMC inventory from IPMI."},
		[]string{"manufacturer", "firmware", "ipmi_version"},
	)
	ServerIPMISensorValue = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "nitrino_server_ipmi_sensor_value", Help: "Numeric local IPMI sensor value."},
		[]string{"sensor", "unit", "status"},
	)
	ServerIPMISensorState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "nitrino_server_ipmi_sensor_state", Help: "State of a local IPMI sensor, including non-numeric sensors."},
		[]string{"sensor", "status", "classification"},
	)
	ServerSystemdFailedUnits = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "nitrino_server_systemd_failed_units", Help: "Number of failed systemd units."},
	)
	ServerDockerContainers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Name: "nitrino_server_docker_containers", Help: "Number of Docker containers grouped by state."},
		[]string{"state"},
	)
)
