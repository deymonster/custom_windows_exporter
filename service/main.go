package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"node_exporter_custom/metrics"
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
func setupLogging() {
	log.SetOutput(os.Stdout)

}

// myService - служба
type myService struct{}

func (m *myService) Execute(args []string, req <-chan svc.ChangeRequest, changes chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	changes <- svc.Status{State: svc.StartPending}

	if wlog != nil {
		wlog.Info(1, "Service started successfully")
	}
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	go func() {
		initMetrics()
		http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			// Check secret key in header
			handshakeKey := r.Header.Get("X-Agent-Handshake-Key")
			if handshakeKey != secretkey {
				clientIP := r.RemoteAddr
				log.Printf("Unauthorized request from IP: %s", clientIP)
				if wlog != nil {
					wlog.Warning(2, fmt.Sprintf("Unauthorized request from IP: %s", clientIP))
				}
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			if wlog != nil {
				wlog.Info(1, fmt.Sprintf("Received authorized request from IP %s", r.RemoteAddr))
			}
			log.Printf("Received authorized request from IP: %s", r.RemoteAddr)
			promhttp.Handler().ServeHTTP(w, r)
		})

		if wlog != nil {
			wlog.Info(1, "Service listening on port 9182")
		}
		log.Println("Listening on :9182")
		log.Fatal(http.ListenAndServe(":9182", nil))
	}()

	for {
		select {
		case c := <-req:
			switch c.Cmd {
			case svc.Stop, svc.Shutdown:
				if wlog != nil {
					wlog.Info(1, "Service stopping")
				}
				changes <- svc.Status{State: svc.StopPending}
				return
			default:
				if wlog != nil {
					wlog.Warning(2, "Received unknown control request")
				}
			}
		case <-time.After(10 * time.Second):
			if wlog != nil {
				wlog.Info(1, "Service running")
			}
		}
	}
}

func initMetrics() {
	log.Println("Initializing metrics...")
	prometheus.MustRegister(metrics.BiosInfo)
	metrics.RecordBiosInfo()

	prometheus.MustRegister(metrics.ProccessCount)
	prometheus.MustRegister(metrics.ProccessMemoryUsage)
	prometheus.MustRegister(metrics.ProccessCPUUsage)
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
	metrics.RecordUUIDMetrics()

	prometheus.MustRegister(metrics.SerialNumberMetric)
	metrics.RecordSNMetrics()

	log.Println("Metrics initialized")
}

func runService(name string, isService bool) {
	if !isService {
		// Интерактивный режим
		log.Println("Running in interactive mode.")
		err := debug.Run(name, &myService{})
		if err != nil {
			log.Fatalln("Error running service in debug mode.", err)
		}
	} else {
		// Службный режим
		log.Println("Running in service  mode.")
		err := svc.Run(name, &myService{})
		if err != nil {
			log.Fatalln("Error running service in Service Control mode.", err)
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

	setupLogging()

	installEventSource()
	setupEventLogger()
	defer func() {
		if wlog != nil {
			wlog.Close()
		}
	}()

	isService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("Failed to check interactive session: %v", err)
	}

	log.Printf("Is isInteractive: %v", isService)

	if !isService {
		log.Println("Running in interactive mode.")
		fmt.Printf("Starting service in interactive mode...\n")
		runService("NITRINOnetControlManager", false)

	} else {
		log.Println("Running as service.")
		runService("NITRINOnetControlManager", true)

	}
}
