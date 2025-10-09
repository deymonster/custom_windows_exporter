package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	ProccessCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_proccess_list",
			Help: "Total number of active proccesses",
		},
	)

	ProccessMemoryUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "active_proccess_memory_usage",
			Help: "Memory usage for each active proccess in MB",
		},
		[]string{"process", "pid"},
	)

	ProccessCPUUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "proccess_cpu_usage_percent",
			Help: "CPU usage for each active proccess",
		},
		[]string{"process", "pid"},
	)

	ProcessInstanceCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_instance_count",
			Help: "Number of instances of each process",
		},
		[]string{"process"},
	)

	ProcessGroupMemoryWorkingSet = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_group_memory_workingset_mb",
			Help: "Total WorkingSet memory usage for all instances of a process in MB",
		},
		[]string{"process", "instances"},
	)

	ProcessGroupMemoryPrivate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_group_memory_private_mb",
			Help: "Total Private memory usage for all instances of a process in MB",
		},
		[]string{"process", "instances"},
	)

	ProcessGroupCPUUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_group_cpu_usage_percent",
			Help: "Total CPU usage for all instances of a process",
		},
		[]string{"process", "instances"},
	)
)
