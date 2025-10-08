//go:build windows

package windows

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"node_exporter_custom/internal/deviceconfig"
	"node_exporter_custom/internal/mockconfig"
	"node_exporter_custom/metrics"
)

type Collector struct {
	mockEnabled      bool
	deviceConfigPath string
}

func New() *Collector {
	return &Collector{
		deviceConfigPath: deviceconfig.DefaultPath(),
	}
}

func (c *Collector) RegisterMetrics(reg prometheus.Registerer) error {
	if err := mockconfig.Load(); err != nil {
		return fmt.Errorf("failed to load mock config: %w", err)
	}

	c.mockEnabled = mockconfig.IsEnabled()

	if c.mockEnabled {
		metrics.LogMockEnabled()
		reg.MustRegister(
			metrics.HardwareUUIDChanged,
			metrics.CpuUsage,
			metrics.CpuTemperature,
			metrics.DiskUsage,
			metrics.DiskUsagePercent,
			metrics.DiskReadBytes,
			metrics.DiskWriteBytes,
			metrics.TotalMemory,
			metrics.UsedMemory,
			metrics.FreeMemory,
			metrics.NetworkErrors,
			metrics.SerialNumberMetric,
		)
		return nil
	}

	reg.MustRegister(metrics.BiosInfo)
	reg.MustRegister(metrics.ProccessCount)
	reg.MustRegister(metrics.ProccessMemoryUsage)
	reg.MustRegister(metrics.ProccessCPUUsage)
	reg.MustRegister(metrics.ProcessInstanceCount)
	reg.MustRegister(metrics.ProcessGroupMemoryWorkingSet)
	reg.MustRegister(metrics.ProcessGroupMemoryPrivate)
	reg.MustRegister(metrics.ProcessGroupCPUUsage)
	reg.MustRegister(metrics.CpuUsage)
	reg.MustRegister(metrics.CpuTemperature)
	reg.MustRegister(metrics.MemoryModuleInfo)
	reg.MustRegister(metrics.TotalMemory)
	reg.MustRegister(metrics.UsedMemory)
	reg.MustRegister(metrics.FreeMemory)
	reg.MustRegister(metrics.DiskUsage)
	reg.MustRegister(metrics.DiskUsagePercent)
	reg.MustRegister(metrics.DiskReadBytes)
	reg.MustRegister(metrics.DiskWriteBytes)
	reg.MustRegister(metrics.DiskHealthStatus)
	reg.MustRegister(metrics.NetworkStatus)
	reg.MustRegister(metrics.NetworkRxBytesPerSecond)
	reg.MustRegister(metrics.NetworkTxBytesPerSecond)
	reg.MustRegister(metrics.NetworkErrors)
	reg.MustRegister(metrics.NetworkDroppedPackets)
	reg.MustRegister(metrics.GpuInfo)
	reg.MustRegister(metrics.GpuMemory)
	reg.MustRegister(metrics.MotherboardInfo)
	reg.MustRegister(metrics.SystemInfo)
	reg.MustRegister(metrics.SystemUptime)
	reg.MustRegister(metrics.SystemUUID)
	reg.MustRegister(metrics.HardwareUUIDChanged)
	reg.MustRegister(metrics.SerialNumberMetric)

	return nil
}

func (c *Collector) Start(ctx context.Context) error {
	go mockconfig.Watch(ctx.Done())

	deviceConfig, err := deviceconfig.Read(c.deviceConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read device config: %w", err)
	}
	metrics.RecordSNMetrics(deviceConfig)

	if err := deviceconfig.Watch(ctx, c.deviceConfigPath, metrics.UpdateSerialNumberMetrics); err != nil {
		return fmt.Errorf("failed to watch device config: %w", err)
	}

	if c.mockEnabled {
		go c.startMockLoop(ctx)
		return nil
	}

	metrics.RecordBiosInfo()
	metrics.RecordProccessInfo()
	metrics.RecordCPUInfo()
	metrics.RecordMemoryModuleInfo()
	metrics.RecordMemoryUsage()
	metrics.RecordDiskUsage()
	metrics.RecordNetworkMetrics()
	metrics.RecordGpuInfo()
	metrics.RecordMotherboardInfo()
	metrics.RecordSystemMetrics()
	metrics.RecordUUIDMetrics()

	return nil
}

func (c *Collector) startMockLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		metrics.ApplyMockConfig(mockconfig.Snapshot())

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
