package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"node_exporter_custom/logmanager"
	"node_exporter_custom/metrics"
	"node_exporter_custom/watcher"

	"node_exporter_custom/internal/api"
	"node_exporter_custom/internal/auth"
	"node_exporter_custom/registryutil"
)

const secretkey = "VERY_SECRET_KEY"

var wlog *eventlog.Log

// Install event source

func installEventSource() {
	err := eventlog.InstallAsEventCreate("NITRINOnetControlManager", eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		log.Printf("Failed to install logger: %v", err)
	} else {
		log.Println("Event source installed")
	}
}

// func removeEventSource() {
// 	err := eventlog.Remove("NITRINOnetControlManager")
// 	if err != nil {
// 		log.Printf("Failed to remove logger: %v", err)
// 	} else {
// 		log.Println("Event source removed")
// 	}
// }

// Setup Event Log
func setupEventLogger() {
	var loggerName = "NITRINOnetControlManager"

	var err error
	wlog, err = eventlog.Open(loggerName)
	if err != nil {
		log.Fatalf("Could not open event log: %v", err)
	} else {
		log.Println("Event log opened")
	}

}

// Setup logging to a file
// func setupLogging() {
// 	logFile, err := os.OpenFile("C:\\ProgramData\\NITRINOnetControlManager\\service.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
// 	if err != nil {
// 		log.Fatalf("Failed to open log file: %v", err)
// 	}
// 	multiWriter := io.MultiWriter(os.Stdout, logFile)
// 	log.SetOutput(multiWriter)

// 	defer func() {
// 		if logFile != nil {
// 			logFile.Sync()
// 			logFile.Close()
// 		}
// 	}()

// }

// myService - служба
type myService struct{}

func (m *myService) Execute(args []string, req <-chan svc.ChangeRequest, changes chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	changes <- svc.Status{State: svc.StartPending}

	if wlog != nil {
		wlog.Info(1, "Service started successfully")
		logmanager.WriteLog("Service started successfully")
	}
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	stopChan := make(chan struct{})
	go startHTTPServer(stopChan)

	for {
		select {
		case c := <-req:
			switch c.Cmd {
			case svc.Stop, svc.Shutdown:
				if wlog != nil {
					wlog.Info(1, "Service stopping")
					logmanager.WriteLog("Service stopping")
				}
				close(stopChan)
				changes <- svc.Status{State: svc.StopPending}
				return
			default:
				if wlog != nil {
					wlog.Warning(2, "Received unknown control request")
					logmanager.WriteLog("Received unknown control request")
				}
			}
		case <-time.After(10 * time.Second):
			if wlog != nil {
				wlog.Info(1, "Service running")
				logmanager.WriteLog("Service running")
			}
		}
	}
}

func startHTTPServer(stopChan chan struct{}) {

	initMetrics()
	// сервер с метриками
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		handshakeKey := r.Header.Get("X-Agent-Handshake-Key")
		if handshakeKey != secretkey {
			clientIP := r.RemoteAddr
			log.Printf("Unauthorized request from IP: %s", clientIP)
			logmanager.WriteLog(fmt.Sprintf("Unauthorized request from IP: %s", clientIP))
			if wlog != nil {
				wlog.Warning(2, fmt.Sprintf("Unauthorized request from IP: %s", clientIP))
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if wlog != nil {
			wlog.Info(1, fmt.Sprintf("Received authorized request from IP %s", r.RemoteAddr))
			logmanager.WriteLog(fmt.Sprintf("Received authorized request from IP %s", r.RemoteAddr))
		}
		log.Printf("Received authorized request from IP: %s", r.RemoteAddr)
		logmanager.WriteLog(fmt.Sprintf("Received authorized request from IP: %s", r.RemoteAddr))
		promhttp.Handler().ServeHTTP(w, r)
	})

	// сервер api
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/update-uuid", func(w http.ResponseWriter, r *http.Request) {
		// 1. Проверка Basic Auth
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}

		// 2. Декодирование Basic Auth
		payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
		if err != nil {
			http.Error(w, "Invalid auth header", http.StatusBadRequest)
			return
		}

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 {
			http.Error(w, "Invalid credentials format", http.StatusBadRequest)
			return
		}

		// 3. Проверка UUID (логина)
		currentUUID, err := api.GetCurrentUUID() // Новая функция в metrics
		if err != nil || pair[0] != currentUUID {
			http.Error(w, "Invalid UUID", http.StatusForbidden)
			return
		}

		// 4. Проверка пароля (просто по хэшу)
		if !auth.IsValid(pair[1]) { // Простая проверка хэша
			http.Error(w, "Invalid password", http.StatusForbidden)
			return
		}

		// 5. Логика обновления UUID
		newUUID, err := metrics.GenerateHardwareUUID()
		if err != nil {
			http.Error(w, "UUID generation failed", http.StatusInternalServerError)
			return
		}

		if err := registryutil.WriteUUIDToRegistry(newUUID); err != nil {
			http.Error(w, "Failed to save UUID", http.StatusInternalServerError)
			return
		}

		metrics.HardwareUUIDChanged.Set(0)
		w.Write([]byte("UUID updated successfully"))
	})

	// Запуск серверов
	//server := &http.Server{Addr: ":9182"}
	metricsServer := &http.Server{Addr: ":9182", Handler: nil} // Default handler
	apiServer := &http.Server{Addr: ":9183", Handler: apiMux}

	go func() {
		log.Println("Starting API server on :9183")
		certPath := filepath.Join(os.Getenv("ProgramData"), "NITRINOnetControlManager", "certs")
		if err := apiServer.ListenAndServeTLS(
			filepath.Join(certPath, "cert.pem"),
			filepath.Join(certPath, "key.pem"),
		); err != nil && err != http.ErrServerClosed {
			log.Printf("API server error: %v", err)
		}
	}()

	// Обработка остановки
	<-stopChan
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := metricsServer.Shutdown(ctx); err != nil {
		log.Printf("Metrics server shutdown error: %v", err)
	}
	if err := apiServer.Shutdown(ctx); err != nil {
		log.Printf("API server shutdown error: %v", err)
	}

}

func initMetrics() {
	log.Println("Initializing metrics...")
	logmanager.WriteLog("Initializing metrics...")
	prometheus.MustRegister(metrics.BiosInfo)
	metrics.RecordBiosInfo()

	prometheus.MustRegister(metrics.ProccessCount)
	prometheus.MustRegister(metrics.ProccessMemoryUsage)
	prometheus.MustRegister(metrics.ProccessCPUUsage)
	prometheus.MustRegister(metrics.ProcessInstanceCount)
	prometheus.MustRegister(metrics.ProcessGroupMemoryWorkingSet)
	prometheus.MustRegister(metrics.ProcessGroupMemoryPrivate)
	prometheus.MustRegister(metrics.ProcessGroupCPUUsage)
	metrics.RecordProccessInfo()

	prometheus.MustRegister(metrics.CpuUsage)
	prometheus.MustRegister(metrics.CpuTemperature)
	metrics.RecordCPUInfo()

	prometheus.MustRegister(metrics.MemoryModuleInfo)
	prometheus.MustRegister(metrics.TotalMemory)
	prometheus.MustRegister(metrics.UsedMemory)
	prometheus.MustRegister(metrics.FreeMemory)
	metrics.RecordMemoryModuleInfo()
	metrics.RecordMemoryUsage()

	prometheus.MustRegister(metrics.DiskUsage)
	prometheus.MustRegister(metrics.DiskUsagePercent)
	prometheus.MustRegister(metrics.DiskReadBytes)
	prometheus.MustRegister(metrics.DiskWriteBytes)
	prometheus.MustRegister(metrics.DiskHealthStatus)
	metrics.RecordDiskUsage()

	prometheus.MustRegister(metrics.NetworkStatus)
	prometheus.MustRegister(metrics.NetworkRxBytesPerSecond)
	prometheus.MustRegister(metrics.NetworkTxBytesPerSecond)
	prometheus.MustRegister(metrics.NetworkErrors)
	prometheus.MustRegister(metrics.NetworkDroppedPackets)
	metrics.RecordNetworkMetrics()

	prometheus.MustRegister(metrics.GpuInfo)
	prometheus.MustRegister(metrics.GpuMemory)
	metrics.RecordGpuInfo()

	prometheus.MustRegister(metrics.MotherboardInfo)
	metrics.RecordMotherboardInfo()

	prometheus.MustRegister(metrics.SystemInfo)
	prometheus.MustRegister(metrics.SystemUptime)
	metrics.RecordSystemMetrics()

	prometheus.MustRegister(metrics.SystemUUID)
	prometheus.MustRegister(metrics.HardwareUUIDChanged)
	metrics.RecordUUIDMetrics()

	prometheus.MustRegister(metrics.SerialNumberMetric)
	metrics.RecordSNMetrics()

	log.Println("Metrics initialized")
	logmanager.WriteLog("Metrics initialized")
}

func runService(name string, isService bool) {
	if !isService {
		// Интерактивный режим
		log.Println("Running in interactive mode.")
		logmanager.WriteLog("Running in interactive mode.")
		err := debug.Run(name, &myService{})
		if err != nil {
			log.Fatalln("Error running service in debug mode.", err)
			logmanager.WriteLog(fmt.Sprintf("Error running service in debug mode: %v", err))
		}
	} else {
		// Службный режим
		log.Println("Running in service  mode.")
		logmanager.WriteLog("Running in service  mode.")
		err := svc.Run(name, &myService{})
		if err != nil {
			log.Fatalln("Error running service in Service Control mode.", err)
			logmanager.WriteLog(fmt.Sprintf("Error running service in Service Control mode: %v", err))
		}
	}
}

// func installService() error {
// 	m, err := mgr.Connect()
// 	if err != nil {
// 		return err
// 	}
// 	defer m.Disconnect()

// 	exePath, err := os.Executable()
// 	if err != nil {
// 		return err
// 	}

// 	config := mgr.Config{
// 		DisplayName: "NITRINOnet Control Manager",
// 		StartType:   mgr.StartAutomatic,
// 		Description: "Система централизованного мониторинга NITRINOnet Control Manager",
// 	}

// 	s, err := m.CreateService("NITRINOnetControlManager", exePath, config)
// 	if err != nil {
// 		return err
// 	}

// 	defer s.Close()

// 	return nil

// }

// func removeService() error {
// 	m, err := mgr.Connect()
// 	if err != nil {
// 		return err
// 	}
// 	defer m.Disconnect()
// 	s, err := m.OpenService("NITRINOnetControlManager")
// 	if err != nil {
// 		return err
// 	}
// 	defer s.Close()
// 	err = s.Delete()
// 	if err != nil {
// 		return err
// 	}
// 	return nil

// }

func main() {

	logFile, err := logmanager.SetupLogging()
	if err != nil {
		log.Fatalf("Failed to setup logging: %v", err)
	}
	defer logmanager.CloseLog(logFile)

	installEventSource()
	setupEventLogger()
	defer func() {
		if wlog != nil {
			wlog.Close()
		}
	}()

	deviceConfig, err := metrics.ReadDeviceConfig()
	if err != nil {
		log.Fatalf("Failed to read device config: %v", err)
		logmanager.WriteLog(fmt.Sprintf("Failed to read device config: %v", err))
	}
	metrics.UpdateSerialNumberMetrics(deviceConfig)

	// Watch config file for changes in a separate goroutine
	go watcher.WatchConfigFile(metrics.ConfigFilePath)

	isService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("Failed to check interactive session: %v", err)
		logmanager.WriteLog(fmt.Sprintf("Failed to check interactive session: %v", err))
	}

	log.Printf("Is isInteractive: %v", isService)
	logmanager.WriteLog(fmt.Sprintf("Is isInteractive: %v", isService))

	if !isService {
		log.Println("Running in interactive mode.")
		fmt.Printf("Starting service in interactive mode...\n")
		logmanager.WriteLog("Starting service in interactive mode...")
		stopChan := make(chan struct{})
		go runService("NITRINOnetControlManager", false)

		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

		<-signalChan

		log.Println("Shutting down...")
		logmanager.WriteLog("Shutting down...")

		close(stopChan)

		time.Sleep(5 * time.Second)

		log.Println("Service stopped.")
		logmanager.WriteLog("Service stopped.")

	} else {
		log.Println("Running as service.")
		logmanager.WriteLog("Running as service.")
		runService("NITRINOnetControlManager", true)

	}

}
