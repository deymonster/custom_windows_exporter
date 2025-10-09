//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"

	"node_exporter_custom/internal/collector"
	"node_exporter_custom/logmanager"
)

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
type myService struct {
	collector collector.Interface
	logger    *serviceLogger
	stopMu    sync.Mutex
	stopChan  chan struct{}
}

func (m *myService) setStopChan(ch chan struct{}) {
	m.stopMu.Lock()
	m.stopChan = ch
	m.stopMu.Unlock()
}

func (m *myService) Stop() {
	m.stopMu.Lock()
	defer m.stopMu.Unlock()
	if m.stopChan != nil {
		close(m.stopChan)
		m.stopChan = nil
	}
}

func (m *myService) Execute(args []string, req <-chan svc.ChangeRequest, changes chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	changes <- svc.Status{State: svc.StartPending}

	if m.logger != nil {
		m.logger.Infof("Service started successfully")
	}
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	stopChan := make(chan struct{})
	m.setStopChan(stopChan)
	go startHTTPServer(stopChan, m.collector, m.logger)

	for {
		select {
		case c := <-req:
			switch c.Cmd {
			case svc.Stop, svc.Shutdown:
				if m.logger != nil {
					m.logger.Infof("Service stopping")
				}
				m.Stop()
				changes <- svc.Status{State: svc.StopPending}
				return
			default:
				if m.logger != nil {
					m.logger.Warnf("Received unknown control request")
				}
			}
		case <-time.After(10 * time.Second):
			if m.logger != nil {
				m.logger.Infof("Service running")
			}
		}
	}
}

func runService(name string, isService bool, handler *myService) {
	logger := handler.logger

	if !isService {
		// Интерактивный режим
		if logger != nil {
			logger.Infof("Running in interactive mode.")
		}
		if err := debug.Run(name, handler); err != nil {
			if logger != nil {
				logger.Errorf("Error running service in debug mode: %v", err)
			}
			os.Exit(1)
		}
	} else {
		// Службный режим
		if logger != nil {
			logger.Infof("Running in service  mode.")
		}
		if err := svc.Run(name, handler); err != nil {
			if logger != nil {
				logger.Errorf("Error running service in Service Control mode: %v", err)
			}
			os.Exit(1)
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
	logOptions := logmanager.DefaultOptions()
	logOptions.EnableFile = shouldEnableFileLogging()

	logMgr, err := logmanager.New(logOptions)
	if err != nil {
		log.Fatalf("Failed to setup logging: %v", err)
	}
	defer func() {
		if err := logMgr.Close(); err != nil {
			log.Printf("Failed to close log manager: %v", err)
		}
	}()

	svcLogger := newServiceLogger(logMgr.Logger())
	if !logOptions.EnableFile && svcLogger != nil {
		svcLogger.Printf("File logging disabled; stdout/stderr will capture service output.")
	}

	collectorImpl, err := collector.New(runtime.GOOS)
	if err != nil {
		if svcLogger != nil {
			svcLogger.Errorf("Failed to initialize collector for %s: %v", runtime.GOOS, err)
		}
		os.Exit(1)
	}

	serviceHandler := &myService{
		collector: collectorImpl,
		logger:    svcLogger,
	}

	isService, err := svc.IsWindowsService()
	if err != nil {
		if svcLogger != nil {
			svcLogger.Errorf("Failed to check interactive session: %v", err)
		}
		os.Exit(1)
	}

	if svcLogger != nil {
		svcLogger.Infof("Is isInteractive: %v", isService)
	}

	if !isService {
		if svcLogger != nil {
			svcLogger.Infof("Running in interactive mode.")
			svcLogger.Printf("Starting service in interactive mode...")
		}
		fmt.Printf("Starting service in interactive mode...\n")
		go runService("NITRINOnetControlManager", false, serviceHandler)

		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

		<-signalChan

		if svcLogger != nil {
			svcLogger.Infof("Shutting down...")
		}

		serviceHandler.Stop()

		time.Sleep(5 * time.Second)

		if svcLogger != nil {
			svcLogger.Infof("Service stopped.")
		}

	} else {
		if svcLogger != nil {
			svcLogger.Infof("Running as service.")
		}
		runService("NITRINOnetControlManager", true, serviceHandler)

	}

}
