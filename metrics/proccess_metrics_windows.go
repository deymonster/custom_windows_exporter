//go:build windows

package metrics

import (
	"fmt"
	"log"
	"sync"
	"time"
	"unsafe"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sys/windows"
)

// Metric declarations live in process_metrics_common.go

type SYSTEM_INFO struct {
	ProcessorArchitecture     uint16
	Reserved                  uint16
	PageSize                  uint32
	MinimumApplicationAddress uintptr
	MaximumApplicationAddress uintptr
	ActiveProcessorMask       uintptr
	NumberOfProcessors        uint32
	ProcessorType             uint32
	AllocationGranularity     uint32
	ProcessorLevel            uint16
	ProcessorRevision         uint16
}

type PROCESSENTRY32 struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [windows.MAX_PATH]uint16
}

type PROCESS_MEMORY_COUNTERS_EX struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uint64
	WorkingSetSize             uint64
	QuotaPeakPagedPoolUsage    uint64
	QuotaPagedPoolUsage        uint64
	QuotaPeakNonPagedPoolUsage uint64
	QuotaNonPagedPoolUsage     uint64
	PagefileUsage              uint64
	PeakPagefileUsage          uint64
	PrivateUsage               uint64
}

type ProcessInfo struct {
	PID       uint32
	Name      string
	RowIndex  int
	Handle    windows.Handle
	HasHandle bool
}

// getProcessList получает список процессов и открывает хэндлы
func getProcessList() ([]ProcessInfo, error) {
	var processes []ProcessInfo

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	var entry PROCESSENTRY32
	entry.Size = uint32(unsafe.Sizeof(entry))

	err = windows.Process32First(snapshot, (*windows.ProcessEntry32)(unsafe.Pointer(&entry)))
	if err != nil {
		return nil, err
	}

	for {
		processID := entry.ProcessID
		exeName := windows.UTF16ToString(entry.ExeFile[:])

		proc := ProcessInfo{
			PID:       processID,
			Name:      exeName,
			HasHandle: false,
		}

		// Не открываем хэндл для системных процессов с PID 0 и 4
		if processID != 0 && processID != 4 {
			handle, err := windows.OpenProcess(
				windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ,
				false,
				processID,
			)

			if err == nil {
				proc.Handle = handle
				proc.HasHandle = true
			}
		}

		processes = append(processes, proc)

		// Получаем следующий процесс
		err = windows.Process32Next(snapshot, (*windows.ProcessEntry32)(unsafe.Pointer(&entry)))
		if err != nil {
			break
		}
	}

	return processes, nil
}

func filetimeToUint64(ft windows.Filetime) uint64 {
	return (uint64(ft.HighDateTime) << 32) | uint64(ft.LowDateTime)
}

func cleanupHandles(processes []ProcessInfo) {
	for _, proc := range processes {
		if proc.HasHandle {
			windows.CloseHandle(proc.Handle)
		}
	}
}

// Безопасно получает системное время
func getSystemTimeSafe() (uint64, error) {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	getSystemTimes := kernel32.NewProc("GetSystemTimes")

	var idleTime, kernelTime, userTime windows.Filetime
	ret, _, err := getSystemTimes.Call(
		uintptr(unsafe.Pointer(&idleTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)

	if ret == 0 {
		return 0, fmt.Errorf("GetSystemTimes failed: %v", err)
	}

	return filetimeToUint64(kernelTime) + filetimeToUint64(userTime), nil
}

func RecordProccessInfo() {
	go func() {
		// Инициализация структур для отслеживания времени
		prevProcessTimes := make(map[uint32]uint64)
		var prevSystemTime uint64
		var mutex sync.Mutex // Для безопасного доступа к prevProcessTimes

		// Инициализация PSAPI
		psapi := windows.NewLazySystemDLL("psapi.dll")
		getProcessMemoryInfo := psapi.NewProc("GetProcessMemoryInfo")

		// Получаем количество процессоров
		kernel32 := windows.NewLazySystemDLL("kernel32.dll")
		getSystemInfo := kernel32.NewProc("GetSystemInfo")
		var si SYSTEM_INFO

		// Вызов GetSystemInfo
		_, _, _ = getSystemInfo.Call(uintptr(unsafe.Pointer(&si)))
		cpuCores := float64(si.NumberOfProcessors)

		// Логируем информацию о системе
		log.Printf("System has %d CPU cores", si.NumberOfProcessors)

		// Счетчик ошибок для GetSystemTimes
		systemTimesErrorCount := 0

		for {
			// Получаем список процессов
			processes, err := getProcessList()
			if err != nil {
				log.Printf("Ошибка получения списка процессов: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// Устанавливаем общее количество процессов
			totalProcesses := len(processes)
			ProccessCount.Set(float64(totalProcesses))
			log.Printf("Total active processes: %d", totalProcesses)

			// Группируем процессы по имени для подсчета экземпляров и суммирования ресурсов
			processGroups := make(map[string][]ProcessInfo)
			for _, proc := range processes {
				processGroups[proc.Name] = append(processGroups[proc.Name], proc)
			}

			// Логируем информацию о группах процессов с несколькими экземплярами
			log.Printf("Process instance counts:")
			for name, procs := range processGroups {
				count := len(procs)
				ProcessInstanceCount.With(prometheus.Labels{
					"process": name,
				}).Set(float64(count))

				if count > 1 {
					log.Printf("  %s: %d instances", name, count)
				}
			}

			// Получение системного времени с обработкой ошибок
			currentSystemTime, err := getSystemTimeSafe()
			if err != nil {
				systemTimesErrorCount++
				log.Printf("GetSystemTimes error (%d occurrences): %v", systemTimesErrorCount, err)

				if systemTimesErrorCount >= 3 {
					log.Printf("Too many GetSystemTimes errors, resetting counters")
					mutex.Lock()
					prevProcessTimes = make(map[uint32]uint64)
					prevSystemTime = 0
					mutex.Unlock()
					systemTimesErrorCount = 0
				}

				time.Sleep(5 * time.Second)
				continue
			}

			systemTimesErrorCount = 0 // Сбрасываем счетчик ошибок при успешном вызове

			// Создаем новую карту для текущих процессов
			currentPIDs := make(map[uint32]bool)

			// Карты для хранения агрегированных данных по группам процессов
			totalMemoryWorkingSet := make(map[string]float64)
			totalMemoryPrivate := make(map[string]float64)
			totalCPU := make(map[string]float64)

			// Обрабатываем каждый процесс
			for _, proc := range processes {
				currentPIDs[proc.PID] = true

				if !proc.HasHandle {
					continue
				}

				// Получение информации о CPU
				var creation, exit, kernel, user windows.Filetime
				err = windows.GetProcessTimes(
					proc.Handle,
					&creation,
					&exit,
					&kernel,
					&user,
				)

				if err != nil {
					continue
				}

				currentProcessTime := filetimeToUint64(kernel) + filetimeToUint64(user)

				// Расчет загрузки CPU
				cpuUsage := 0.0
				mutex.Lock()
				if prevSystemTime > 0 && prevProcessTimes[proc.PID] > 0 {
					timeDelta := currentSystemTime - prevSystemTime
					processDelta := currentProcessTime - prevProcessTimes[proc.PID]

					if timeDelta > 0 {
						cpuUsage = (float64(processDelta) / float64(timeDelta)) * 100.0

						// Нормализуем по количеству ядер
						if cpuUsage > 0 {
							cpuUsage = cpuUsage / cpuCores
						}

						// Ограничиваем максимальное значение до 100%
						if cpuUsage > 100.0 {
							cpuUsage = 100.0
						}
					}
				}
				prevProcessTimes[proc.PID] = currentProcessTime
				mutex.Unlock()

				// Получение информации о памяти
				var memInfo PROCESS_MEMORY_COUNTERS_EX
				memInfo.CB = uint32(unsafe.Sizeof(memInfo))
				ret, _, _ := getProcessMemoryInfo.Call(
					uintptr(proc.Handle),
					uintptr(unsafe.Pointer(&memInfo)),
					uintptr(memInfo.CB),
				)

				// Преобразуем байты в мегабайты
				var workingSetMB, privateMB float64

				if ret == 0 {
					// Если не удалось получить информацию о памяти, используем нулевые значения
					// но продолжаем обработку процесса
					workingSetMB = 0
					privateMB = 0
				} else {
					workingSetMB = float64(memInfo.WorkingSetSize) / (1024 * 1024)
					privateMB = float64(memInfo.PrivateUsage) / (1024 * 1024)
				}

				// Суммируем ресурсы по группам процессов
				totalMemoryWorkingSet[proc.Name] += workingSetMB
				totalMemoryPrivate[proc.Name] += privateMB
				totalCPU[proc.Name] += cpuUsage

				// Логируем только для важных процессов или с высоким использованием ресурсов
				if cpuUsage > 0.5 || workingSetMB > 100.0 {
					log.Printf("Process: %s (PID: %d) - Memory: WorkingSet=%.2f MB, Private=%.2f MB, CPU: %.2f%%",
						proc.Name,
						proc.PID,
						workingSetMB,
						privateMB,
						cpuUsage)
				}

				// Устанавливаем метрики для каждого процесса только если есть реальные данные
				// (чтобы не засорять Prometheus нулевыми значениями)
				if workingSetMB > 0 || privateMB > 0 || cpuUsage > 0 {
					ProccessMemoryUsage.With(prometheus.Labels{
						"process": proc.Name,
						"pid":     fmt.Sprint(proc.PID),
					}).Set(workingSetMB) // Используем WorkingSetSize как в Task Manager

					ProccessCPUUsage.With(prometheus.Labels{
						"process": proc.Name,
						"pid":     fmt.Sprint(proc.PID),
					}).Set(cpuUsage)
				}
			}

			// Логируем агрегированные данные для групп процессов с несколькими экземплярами
			log.Printf("Aggregated process resource usage:")
			for name, procs := range processGroups {
				instanceCount := len(procs)

				// Устанавливаем метрики для всех процессов, даже с одним экземпляром
				ProcessGroupMemoryWorkingSet.With(prometheus.Labels{
					"process":   name,
					"instances": fmt.Sprint(instanceCount),
				}).Set(totalMemoryWorkingSet[name])

				ProcessGroupMemoryPrivate.With(prometheus.Labels{
					"process":   name,
					"instances": fmt.Sprint(instanceCount),
				}).Set(totalMemoryPrivate[name])

				ProcessGroupCPUUsage.With(prometheus.Labels{
					"process":   name,
					"instances": fmt.Sprint(instanceCount),
				}).Set(totalCPU[name])

				// Логируем только процессы с несколькими экземплярами или значительным использованием ресурсов
				if instanceCount > 1 || totalMemoryWorkingSet[name] > 50 || totalCPU[name] > 1.0 {
					log.Printf("  %s (%d instances) - Total Memory: WorkingSet=%.2f MB, Private=%.2f MB, Total CPU: %.2f%%",
						name,
						instanceCount,
						totalMemoryWorkingSet[name],
						totalMemoryPrivate[name],
						totalCPU[name])
				}
			}

			// Очищаем prevProcessTimes от завершенных процессов
			mutex.Lock()
			for pid := range prevProcessTimes {
				if !currentPIDs[pid] {
					delete(prevProcessTimes, pid)
				}
			}
			prevSystemTime = currentSystemTime
			mutex.Unlock()

			// Закрываем хэндлы процессов
			cleanupHandles(processes)

			// Ждем перед следующим обновлением
			time.Sleep(5 * time.Second)
		}
	}()
}
