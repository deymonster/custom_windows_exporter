package metrics

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"node_exporter_custom/registryutil"

	"github.com/StackExchange/wmi"
	"github.com/prometheus/client_golang/prometheus"
)

// Структуры для хранения информации о системе и операционной системе
type Win32_ComputerSystem_UUID struct {
	Manufacturer string
	Model        string
}

type Win32_NetworkAdapter_UUID struct {
	MACAddress string
}

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

// generateHardwareUUID generates a unique hardware UUID for the system by
// collecting various hardware and system information. It retrieves data from
// the BIOS, CPU, physical disks, computer system, network adapters,
// motherboard, memory modules, and GPUs. The function constructs a string
// containing all collected information, generates a SHA-256 hash from it, and
// formats the hash into a UUID-like string. If any information retrieval fails,
// it returns an error.

func generateHardwareUUID() (string, error) {
	var sb strings.Builder

	// 1. Получаем информацию о BIOS
	biosData, err := GetBiosInfo()
	if err != nil || len(biosData) == 0 {
		return "", fmt.Errorf("error getting BIOS info: %v", err)
	}
	bios := biosData[0]
	sb.WriteString(fmt.Sprintf("%s|%s|%s", bios.Manufacturer, bios.Version, bios.ReleaseDate))

	// 2. Получаем информацию о процессоре
	cpuInfo, err := GetCPUInfo()
	if err != nil || len(cpuInfo) == 0 {
		return "", fmt.Errorf("error getting CPU info: %v", err)
	}
	cpu := cpuInfo[0]
	sb.WriteString(fmt.Sprintf("|%s|%d", cpu.Name, cpu.NumberOfLogicalProcessors))

	// 3. Получаем информацию о физических дисках
	disks, err := GetPhysicalDisks()
	if err != nil {
		return "", fmt.Errorf("error getting disks info: %v", err)
	}
	for _, disk := range disks {
		sb.WriteString(fmt.Sprintf("|%s|%s|%d", disk.FriendlyName, disk.SerialNumber, disk.Size))
	}

	// 4. Получаем информацию о системе
	var computerSystem []Win32_ComputerSystem_UUID
	err = wmi.Query("SELECT Manufacturer, Model FROM Win32_ComputerSystem", &computerSystem)
	if err != nil || len(computerSystem) == 0 {
		return "", fmt.Errorf("error getting system info: %v", err)
	}
	cs := computerSystem[0]
	sb.WriteString(fmt.Sprintf("|%s|%s", cs.Manufacturer, cs.Model))

	// 5. Получаем MAC-адрес основного сетевого адаптера
	adapters, err := GetPhysicalNetworkAdapters()
	if err != nil || len(adapters) == 0 {
		return "", fmt.Errorf("error getting network adapters: %v", err)
	}
	sb.WriteString(fmt.Sprintf("|%s", adapters[0].MACAddress))

	// 6. Получаем информацию о материнской плате
	mb, err := GetMotherboardInfo()
	if err != nil {
		return "", fmt.Errorf("error getting motherboard info: %v", err)
	}
	sb.WriteString(fmt.Sprintf("|%s|%s|%s", mb.Manufacturer, mb.Product, mb.SerialNumber))

	// 7. Получаем информацию о модулях памяти
	memModules, err := GetMemoryModules()
	if err != nil {
		return "", fmt.Errorf("error getting memory modules: %v", err)
	}
	for _, module := range memModules {
		sb.WriteString(fmt.Sprintf("|%s|%s|%s|%d|%d",
			module.Manufacturer,
			module.PartNumber,
			module.SerialNumber,
			module.Capacity,
			module.Speed))
	}

	// 8. Получаем информацию о видеокартах
	gpus, err := GetGPUInfo()
	if err != nil {
		return "", fmt.Errorf("error getting GPU info: %v", err)
	}
	for _, gpu := range gpus {
		sb.WriteString(fmt.Sprintf("|%s|%d", gpu.Name, gpu.AdapterRAM))
	}

	// Генерируем хеш SHA-256 из собранных данных
	combinedInfo := sb.String()
	hash := sha256.Sum256([]byte(combinedInfo))
	hashStr := hex.EncodeToString(hash[:])

	// Форматируем в UUID-подобную строку
	uuid := fmt.Sprintf("%s-%s-%s-%s-%s",
		hashStr[0:8],
		hashStr[8:12],
		"4"+hashStr[13:16], // версия 4
		"8"+hashStr[17:20], // вариант 1
		hashStr[20:32],
	)

	return uuid, nil

}

// RecordUUIDMetrics runs in a separate goroutine and records the unique hardware
// UUID of the system into a Prometheus metric. It generates the UUID by collecting
// various hardware and system information, and updates the metric with the UUID.
// The function logs an error if the UUID generation fails.

func RecordUUIDMetrics() {
	go func() {
		currentUUID, err := generateHardwareUUID()
		if err != nil {
			log.Printf("Failed to generate hardware UUID: %v", err)
			return
		}

		// Проверяем существование ключа
		exists, err := registryutil.KeyExists()
		if err != nil {
			log.Printf("Error checking registry key: %v", err)
			return
		}

		if !exists {
			// Создаем ключ если не существует
			if err := registryutil.CreateKey(); err != nil {
				log.Printf("Failed to create registry key: %v", err)
				return
			}

			// Записываем UUID в реестр
			if err := registryutil.WriteUUIDToRegistry(currentUUID); err != nil {
				log.Printf("Failed to write initial UUID to registry: %v", err)
				return
			}
			log.Println("Created new registry key and stored initial UUID")
			HardwareUUIDChanged.Set(0)
		} else {
			// Сравниваем с сохраненным UUID
			storedUUID, err := registryutil.ReadUUIDFromRegistry()
			if err != nil {
				log.Printf("Failed to read UUID from registry: %v", err)
				HardwareUUIDChanged.Set(0)
				return
			}

			if storedUUID != currentUUID {

				// Устанавливаем флаг изменения
				HardwareUUIDChanged.Set(1)
				log.Printf("Hardware UUID changed! Old: %s, New: %s", storedUUID, currentUUID)
			} else {
				HardwareUUIDChanged.Set(0)
			}
		}
		// Записываем UUID в метрику
		SystemUUID.With(prometheus.Labels{
			"uuid": currentUUID,
		}).Set(1)

	}()
}
