package metrics

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

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

// Функция для получения аппаратных данных и генерации UUID
func RecordUUIDMetrics() {
	go func() {
		uuid, err := generateHardwareUUID()
		if err != nil {
			log.Printf("Failed to generate hardware UUID: %v", err)
			return
		}

		SystemUUID.With(prometheus.Labels{
			"uuid": uuid,
		}).Set(1)

	}()
}
