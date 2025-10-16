//go:build windows

package metrics

import (
	"fmt"
	"log"
	"time"

	"github.com/StackExchange/wmi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/disk"
)

type MSFT_PhysicalDisk struct {
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

// GetPhysicalDisks retrieves information about physical disks in the system
// by querying the MSFT_PhysicalDisk WMI class. It returns a slice of
// MSFT_PhysicalDisk structs containing details such as friendly name,
// serial number, media type, health status, and size, or an error if the
// query fails.
func GetPhysicalDisks() ([]MSFT_PhysicalDisk, error) {
	var physicalDisks []MSFT_PhysicalDisk
	err := wmi.QueryNamespace(
		"SELECT FriendlyName, SerialNumber, MediaType, HealthStatus, Size FROM MSFT_PhysicalDisk",
		&physicalDisks,
		"ROOT\\Microsoft\\Windows\\Storage",
	)
	if err != nil {
		return nil, fmt.Errorf("error getting physical disks info: %v", err)
	}
	return physicalDisks, nil
}

// GetLogicalDisks retrieves information about logical disks in the system
// by querying the Win32_LogicalDisk WMI class. It returns a slice of
// Win32_LogicalDisk structs containing details such as device ID, size,
// free space, and file system type, or an error if the query fails.

func GetLogicalDisks() ([]Win32_LogicalDisk, error) {
	var logicalDisks []Win32_LogicalDisk
	err := wmi.Query(
		"SELECT DeviceID, Size, FreeSpace, FileSystem FROM Win32_LogicalDisk WHERE DriveType = 3",
		&logicalDisks,
	)
	if err != nil {
		return nil, fmt.Errorf("error getting logical disk info: %v", err)
	}
	return logicalDisks, nil
}

// GetDiskIOCounters retrieves the disk I/O counters for a specific device
// identified by deviceID. It returns a disk.IOCountersStat struct containing
// the I/O statistics for the device, or an error if the statistics cannot
// be retrieved. If no I/O stats are found for the specified device, an error
// is returned.

func GetDiskIOCounters(deviceID string) (disk.IOCountersStat, error) {
	ioStat, err := disk.IOCounters(deviceID)
	if err != nil {
		return disk.IOCountersStat{}, fmt.Errorf("error getting IO stats for %s: %v", deviceID, err)
	}
	if stat, ok := ioStat[deviceID]; ok {
		return stat, nil
	}
	return disk.IOCountersStat{}, fmt.Errorf("no IO stats found for %s", deviceID)
}

// mediaTypeToString takes a media type code and returns a human-readable
// string corresponding to the media type. The mapping is as follows:
//
//	3: Hard Disk Drive (HDD)
//	4: Solid-State Drive (SSD)
//
// Any other value is returned as "Unknown".
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

// healthStatusToString takes a health status code and returns a human-readable
// string corresponding to the health status of the disk. The mapping is as
// follows:
//
//	0: Healthy
//	1: Warning
//	2: Unhealthy
//
// Any other value is returned as "Unknown".
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

// RecordDiskUsage starts a goroutine which records disk usage metrics on a
// regular schedule. It queries the MSFT_PhysicalDisk WMI class to get a list
// of physical disks and their media types, and then queries the diskutil
// library to get the current disk usage and IO counters for each physical
// disk. It then records the following metrics:
//
// * disk_usage_bytes: The total, used, and free space on each disk
// * disk_usage_percent: The percentage of used space on each disk
// * disk_read_bytes_per_second: The read speed of each disk
// * disk_write_bytes_per_second: The write speed of each disk
// * disk_health_status: The health status of each physical disk
//
// The metrics are recorded in a goroutine which runs every 5 seconds.
func RecordDiskUsage() {
	go func() {

		var prevIO = make(map[string]disk.IOCountersStat)
		modelMap := make(map[string]string)

		// Получаем информацию о физических дисках один раз при старте
		physicalDisks, err := GetPhysicalDisks()
		if err != nil {
			log.Printf("%v", err)
			return
		}

		// Создаем маппинг имен дисков к их описаниям
		for _, drive := range physicalDisks {
			mediaTypeStr := mediaTypeToString(drive.MediaType)
			healthStatusStr := healthStatusToString(drive.HealthStatus)
			log.Printf("Detected physical disk: FriendlyName=%s, SerialNumber=%s, MediaType=%s, Size=%d",
				drive.FriendlyName, drive.SerialNumber, mediaTypeStr, drive.Size)

			modelMap[drive.FriendlyName] = fmt.Sprintf("%s (SN: %s, Type: %s, Health: %s, Size: %d)",
				drive.FriendlyName, drive.SerialNumber, mediaTypeStr, healthStatusStr, drive.Size)

			// Записываем метрику здоровья диска
			healthValue := 1.0
			if healthStatusStr != "Healthy" {
				healthValue = 0.0
			}
			DiskHealthStatus.With(prometheus.Labels{
				"disk":   drive.FriendlyName,
				"serial": drive.SerialNumber,
				"type":   mediaTypeStr,
				"status": healthStatusStr,
				"size":   fmt.Sprintf("%d", drive.Size),
			}).Set(healthValue)
		}

		for {
			// Получаем информацию о логических дисках
			partitions, err := GetLogicalDisks()
			if err != nil {
				log.Printf("%v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			for _, part := range partitions {
				model := modelMap[part.DeviceID]

				// Записываем метрики использования диска
				DiskUsage.With(prometheus.Labels{
					"disk":   part.DeviceID,
					"model":  model,
					"serial": "unknown",
					"type":   "total",
				}).Set(float64(part.Size))

				DiskUsage.With(prometheus.Labels{
					"disk":   part.DeviceID,
					"model":  model,
					"serial": "unknown",
					"type":   "free",
				}).Set(float64(part.FreeSpace))

				DiskUsage.With(prometheus.Labels{
					"disk":   part.DeviceID,
					"model":  model,
					"serial": "unknown",
					"type":   "used",
				}).Set(float64(part.Size - part.FreeSpace))

				usedPercent := (float64(part.Size-part.FreeSpace) / float64(part.Size)) * 100
				DiskUsagePercent.With(prometheus.Labels{
					"disk":   part.DeviceID,
					"model":  model,
					"serial": "unknown",
				}).Set(usedPercent)

				// Получаем и записываем метрики IO
				current, err := GetDiskIOCounters(part.DeviceID)
				if err != nil {
					log.Printf("%v", err)
					continue
				}

				if prev, ok := prevIO[part.DeviceID]; ok {
					duration := 5.0 // Период обновления в секундах
					readSpeed := float64(current.ReadBytes-prev.ReadBytes) / duration
					writeSpeed := float64(current.WriteBytes-prev.WriteBytes) / duration

					DiskReadBytes.With(prometheus.Labels{
						"disk":   part.DeviceID,
						"model":  model,
						"serial": "unknown",
					}).Set(readSpeed)

					DiskWriteBytes.With(prometheus.Labels{
						"disk":   part.DeviceID,
						"model":  model,
						"serial": "unknown",
					}).Set(writeSpeed)
				}
				prevIO[part.DeviceID] = current
			}
			time.Sleep(5 * time.Second)
		}

	}()
}
