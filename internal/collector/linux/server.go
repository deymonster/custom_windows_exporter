//go:build linux

package linux

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"node_exporter_custom/metrics"
)

func resolveProfile(configProfile string) string {
	profile := strings.ToLower(strings.TrimSpace(os.Getenv("NCM_PROFILE")))
	if profile == "" {
		profile = strings.ToLower(strings.TrimSpace(configProfile))
	}
	switch profile {
	case "desktop", "server":
		return profile
	case "", "auto":
		if _, err := os.Stat("/dev/ipmi0"); err == nil {
			return "server"
		}
		return "desktop"
	default:
		return "desktop"
	}
}

func registerServerMetrics(reg prometheus.Registerer) {
	reg.MustRegister(
		metrics.ServerProfileInfo,
		metrics.ServerDMIInfo,
		metrics.ServerIPMIInfo,
		metrics.ServerIPMISensorValue,
		metrics.ServerIPMISensorState,
		metrics.ServerSystemdFailedUnits,
		metrics.ServerDockerContainers,
	)
}

func recordServerMetrics(ctx context.Context) {
	recordServerMetricsOnce(ctx)
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			recordServerMetricsOnce(ctx)
		}
	}
}

func recordServerMetricsOnce(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	recordDMI(ctx)
	recordIPMI(ctx)
	recordSystemd(ctx)
	recordDocker(ctx)
}

func recordDMI(ctx context.Context) {
	output, err := runCommand(ctx, "dmidecode", "-t", "system", "-t", "baseboard")
	if err != nil {
		return
	}
	values := dmiValues(string(output))
	metrics.ServerDMIInfo.With(prometheus.Labels{
		"system_vendor": valueOrUnknown(values["system_manufacturer"]),
		"system_model":  valueOrUnknown(values["system_product"]),
		"board_vendor":  valueOrUnknown(values["board_manufacturer"]),
		"board_model":   valueOrUnknown(values["board_product"]),
		"board_serial":  valueOrUnknown(values["board_serial"]),
	}).Set(1)
}

func dmiValues(output string) map[string]string {
	values := map[string]string{}
	section := ""
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch line {
		case "System Information":
			section = "system"
		case "Base Board Information":
			section = "board"
		}
		if section == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if value == "" || strings.EqualFold(value, "to be filled by o.e.m.") {
			continue
		}
		switch key {
		case "Manufacturer":
			values[section+"_manufacturer"] = value
		case "Product Name":
			values[section+"_product"] = value
		case "Serial Number":
			values[section+"_serial"] = value
		}
	}
	return values
}

func recordIPMI(ctx context.Context) {
	info, err := runCommand(ctx, "ipmitool", "mc", "info")
	if err != nil {
		return
	}
	values := colonValues(string(info))
	metrics.ServerIPMIInfo.With(prometheus.Labels{
		"manufacturer": valueOrUnknown(values["Manufacturer Name"]),
		"firmware":     valueOrUnknown(values["Firmware Revision"]),
		"ipmi_version": valueOrUnknown(values["IPMI Version"]),
	}).Set(1)

	sensors, err := runCommand(ctx, "ipmitool", "sdr", "elist")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(sensors), "\n") {
		fields := strings.Split(line, "|")
		if len(fields) < 5 {
			continue
		}
		name := strings.TrimSpace(fields[0])
		status := strings.ToLower(strings.TrimSpace(fields[2]))
		reading := strings.TrimSpace(fields[len(fields)-1])
		if name == "" || status == "" {
			continue
		}
		classification := classifySensorReading(status, reading)
		// Reading is intentionally not a label: a changing value in a Prometheus
		// label would create an unbounded number of time series. Numeric readings
		// are exposed separately through ServerIPMISensorValue.
		metrics.ServerIPMISensorState.With(prometheus.Labels{"sensor": name, "status": status, "classification": classification}).Set(1)
		if value, unit, ok := parseSensorReading(reading); ok {
			metrics.ServerIPMISensorValue.With(prometheus.Labels{"sensor": name, "unit": unit, "status": status}).Set(value)
		}
	}
}

func classifySensorReading(status, reading string) string {
	// "ns" is reported by some BMCs for a sensor whose reading is not
	// available. It is not evidence of a hardware fault and must never page an
	// administrator on its own.
	if status == "ns" || status == "na" {
		return "unavailable"
	}
	value, _, numeric := parseSensorReading(reading)
	if numeric && value == 0 && (status == "lnr" || status == "lnc" || status == "lcr") {
		// A zero-valued lower-threshold sensor is often an unpopulated fan or
		// thermal header. It must be baselined by an administrator before it
		// creates a critical alert; otherwise server boards produce false alarms.
		return "needs_baseline"
	}
	if status == "ok" {
		return "healthy"
	}
	return "fault"
}

func parseSensorReading(reading string) (float64, string, bool) {
	fields := strings.Fields(reading)
	if len(fields) < 2 {
		return 0, "", false
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, "", false
	}
	return value, strings.ToLower(fields[1]), true
}

func recordSystemd(ctx context.Context) {
	output, err := runCommand(ctx, "systemctl", "--failed", "--no-legend", "--no-pager")
	if err != nil {
		return
	}
	count := 0
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	metrics.ServerSystemdFailedUnits.Set(float64(count))
}

func recordDocker(ctx context.Context) {
	output, err := runCommand(ctx, "docker", "ps", "-a", "--format", "{{.State}}")
	if err != nil {
		return
	}
	states := map[string]int{}
	for _, line := range strings.Split(string(output), "\n") {
		if state := strings.TrimSpace(line); state != "" {
			states[state]++
		}
	}
	for state, count := range states {
		metrics.ServerDockerContainers.With(prometheus.Labels{"state": state}).Set(float64(count))
	}
}

func colonValues(output string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return values
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func runCommand(ctx context.Context, command string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, command, args...).Output()
}
