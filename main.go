package main

import (
	"fmt"
	"io"
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

// Setup logging to a file
func setupLogging() (*os.File, error) {
	f, err := os.OpenFile("service.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	log.SetOutput(f)

	return f, nil
}

// myService - служба
type myService struct{}

func (m *myService) Execute(args []string, req <-chan svc.ChangeRequest, changes chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	changes <- svc.Status{State: svc.StartPending}

	elog, err := eventlog.Open("NITRINOnetControlManager")
	if err != nil {
		log.Fatalf("Could not open event log: %v", err)
	}
	defer elog.Close()

	elog.Info(1, "Service started")
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	go func() {
		initMetrics()
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Listening on :9183")
		log.Fatal(http.ListenAndServe(":9183", nil))
	}()

	for {
		select {
		case c := <-req:
			switch c.Cmd {
			case svc.Stop, svc.Shutdown:
				elog.Info(1, "Service stopping")
				changes <- svc.Status{State: svc.StopPending}
				return
			default:
				elog.Warning(1, "Received unknown command")
			}
		case <-time.After(10 * time.Second):
			elog.Info(1, "Service running")
		}
	}
}

func initMetrics() {

	prometheus.MustRegister(metrics.BiosInfo)

	prometheus.MustRegister(metrics.ProccessCount)
	prometheus.MustRegister(metrics.ProccessMemoryUsage)
	prometheus.MustRegister(metrics.ProccessCPUUsage)

	prometheus.MustRegister(metrics.CpuUsage)
	prometheus.MustRegister(metrics.CpuTemperature)

	prometheus.MustRegister(metrics.MemoryModuleInfo)
	prometheus.MustRegister(metrics.TotalMemory)
	prometheus.MustRegister(metrics.UsedMemory)
	prometheus.MustRegister(metrics.FreeMemory)

	prometheus.MustRegister(metrics.DiskUsage)
	prometheus.MustRegister(metrics.DiskUsagePercent)
	prometheus.MustRegister(metrics.DiskReadBytes)
	prometheus.MustRegister(metrics.DiskWriteBytes)
	prometheus.MustRegister(metrics.DiskHealthStatus)

	prometheus.MustRegister(metrics.NetworkStatus)
	prometheus.MustRegister(metrics.NetworkRxBytesPerSecond)
	prometheus.MustRegister(metrics.NetworkTxBytesPerSecond)
	prometheus.MustRegister(metrics.NetworkErrors)
	prometheus.MustRegister(metrics.NetworkDroppedPackets)

	prometheus.MustRegister(metrics.GpuInfo)
	prometheus.MustRegister(metrics.GpuMemory)

	prometheus.MustRegister(metrics.MotherboardInfo)

	prometheus.MustRegister(metrics.SystemInfo)
	prometheus.MustRegister(metrics.SystemUptime)
}

func runService(name string, isDebug bool) {
	if isDebug {
		err := debug.Run(name, &myService{})
		if err != nil {
			log.Fatalln("Error running service in debug mode.")
		}
	} else {
		err := svc.Run(name, &myService{})
		if err != nil {
			log.Fatalln("Error running service in Service Control mode.")
		}
	}
}

func main() {
	logFile, err := setupLogging()
	if err != nil {
		log.Fatalf("Failed to setup logging: %v", err)
	}

	defer logFile.Close()

	isInteractive, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("Failed to check interactive session: %v", err)
	}

	log.Printf("Is running as service: %v", !isInteractive)

	if isInteractive {
		log.Println("Running in interactive mode.")

		fmt.Printf("Starting service in interactive mode...\n")
		runService("NITRINOnetControlManager", false)

	} else {

		runService("NITRINOnetControlManager", true)

	}
}
