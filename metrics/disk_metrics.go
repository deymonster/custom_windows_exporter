package metrics

import (
	"fmt"
	"log"
	"time"

	"github.com/StackExchange/wmi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/disk"
)

type MSF_PhysicalDisk struct {
	FriendlyName string
	SerialNumber string
	MediaType    uint16
	HealthStatus uint16
	Size         uint64
}

type Win32_LogicalDisk struct {
	DeviceID   string
	Size       uint64
	FreeSpace  uint64
	FileSystem string
}

var (
	// Disk usage
	DiskUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_usage_bytes",
			Help: "Disk usage on system",
		},
		[]string{"disk", "model", "type"},
	)

	DiskUsagePercent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_usage_percent",
			Help: "Disk usage on system",
		},
		[]string{"disk", "model"},
	)

	DiskReadBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_read_bytes_per_second",
			Help: "Disk read bytes per second",
		},
		[]string{"disk", "model"},
	)

	DiskWriteBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_write_bytes_per_second",
			Help: "Disk write bytes per second",
		},
		[]string{"disk", "model"},
	)

	DiskHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "disk_health_status",
			Help: "Health status of disk",
		},
		[]string{"disk", "type", "status", "size"},
	)
)

func wmiQueryLogicalDisks() ([]Win32_LogicalDisk, error) {
	var logicalDisks []Win32_LogicalDisk
	err := wmi.Query("SELECT DeviceID, Size, FreeSpace, FileSystem FROM Win32_LogicalDisk WHERE DriveType = 3", &logicalDisks)
	if err != nil {
		log.Printf("Error getting logical disk info: %v", err)
		return nil, fmt.Errorf("error getting logical disk info: %v", err)
	}
	return logicalDisks, nil
}

func mediaTypeToString(mediaType uint16) string {
	switch mediaType {
	case 3:
		return "HDD"
	case 4:
		return "SSD"
	default:
		return "Unkown"
	}
}

func healthStatusToString(status uint16) string {
	switch status {
	case 0:
		return "Healthy"
	case 1:
		return "Warning"
	case 2:
		return "Unhealthy"
	default:
		return "Unknown"
	}
}

func RecordDiskUsage() {
	go func() {

		var prevIO = make(map[string]disk.IOCountersStat)
		var physicalDisks []MSF_PhysicalDisk

		// Запрос модели, серийного номера, типаб статусовб размера физических дисков
		err := wmi.QueryNamespace("SELECT FriendlyName, SerialNumber, MediaType, HealthStatus, Size FROM MSFT_PhysicalDisk",
			&physicalDisks, "ROOT\\Microsoft\\Windows\\Storage")
		if err != nil {
			log.Printf("Error getting info about physical disks: %v", err)
			return
		}

		modelMap := make(map[string]string)
		for _, drive := range physicalDisks {
			mediaTypeStr := mediaTypeToString(drive.MediaType)
			healthStatusStr := healthStatusToString(drive.HealthStatus)
			log.Printf("Detected physical disk: FriendlyName=%s, SerialNumber=%s, MediaType=%s, Size=%d",
				drive.FriendlyName, drive.SerialNumber, mediaTypeStr, drive.Size)
			modelMap[drive.FriendlyName] = fmt.Sprintf("%s (SN: %s, Type: %s, Health: %s, Size: %d)", drive.FriendlyName, drive.SerialNumber, mediaTypeStr, healthStatusStr, drive.Size)
			healthValue := 1.0
			if healthStatusStr != "Healthy" {
				healthValue = 0.0
			}
			DiskHealthStatus.With(prometheus.Labels{
				"disk":   drive.FriendlyName,
				"type":   mediaTypeStr,
				"status": healthStatusStr,
				"size":   fmt.Sprintf("%d", drive.Size),
			}).Set(healthValue)
		}

		for {
			partitions, err := wmiQueryLogicalDisks()
			if err != nil {
				log.Printf("Error getting disk partitions: %v", err)
				continue
			}

			for _, part := range partitions {

				model := modelMap[part.DeviceID]

				// Общее, свободное и использованное пространство
				DiskUsage.With(prometheus.Labels{"disk": part.DeviceID, "model": model, "type": "total"}).Set(float64(part.Size))
				DiskUsage.With(prometheus.Labels{"disk": part.DeviceID, "model": model, "type": "free"}).Set(float64(part.FreeSpace))
				DiskUsage.With(prometheus.Labels{"disk": part.DeviceID, "model": model, "type": "used"}).Set(float64(part.Size - part.FreeSpace))
				usedPercent := (float64(part.Size-part.FreeSpace) / float64(part.Size)) * 100
				DiskUsagePercent.With(prometheus.Labels{"disk": part.DeviceID, "model": model}).Set(usedPercent)

				// Скорость чтения и записи
				ioStat, err := disk.IOCounters(part.DeviceID)
				if err == nil && len(ioStat) > 0 {
					current := ioStat[part.DeviceID]

					if prev, ok := prevIO[part.DeviceID]; ok {
						duration := 5.0 // Период обновления в секундах
						readSpeed := float64(current.ReadBytes-prev.ReadBytes) / duration
						writeSpeed := float64(current.WriteBytes-prev.WriteBytes) / duration
						DiskReadBytes.With(prometheus.Labels{"disk": part.DeviceID, "model": model}).Set(readSpeed)
						DiskWriteBytes.With(prometheus.Labels{"disk": part.DeviceID, "model": model}).Set(writeSpeed)
					}
					prevIO[part.DeviceID] = current
				} else {
					log.Printf("Error getting IO stats for %s: %v", part.DeviceID, err)
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()
}
