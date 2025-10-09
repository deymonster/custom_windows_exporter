package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"node_exporter_custom/internal/api"
	"node_exporter_custom/internal/collector"
	"node_exporter_custom/logmanager"
)

const secretkey = "VERY_SECRET_KEY"

type serviceLogger struct {
	logger *log.Logger
}

func newServiceLogger(base *log.Logger) *serviceLogger {
	return &serviceLogger{logger: base}
}

func (l *serviceLogger) Infof(format string, args ...interface{}) {
	l.log(format, args...)
}

func (l *serviceLogger) Warnf(format string, args ...interface{}) {
	l.log(format, args...)
}

func (l *serviceLogger) Errorf(format string, args ...interface{}) {
	l.log(format, args...)
}

func (l *serviceLogger) Printf(format string, args ...interface{}) {
	if l == nil || l.logger == nil {
		return
	}
	if len(args) > 0 {
		l.logger.Printf(format, args...)
	} else {
		l.logger.Println(format)
	}
}

func (l *serviceLogger) log(format string, args ...interface{}) {
	if l == nil || l.logger == nil {
		return
	}

	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	l.logger.Println(msg)
}

func parseBoolEnv(value string) (bool, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false, false
	}

	switch strings.ToLower(trimmed) {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	default:
		return false, false
	}
}

func shouldEnableFileLogging() bool {
	if enabledValue, ok := parseBoolEnv(os.Getenv("NCM_ENABLE_FILE_LOG")); ok {
		return enabledValue
	}

	if disabledValue, ok := parseBoolEnv(os.Getenv("NCM_DISABLE_FILE_LOG")); ok {
		return !disabledValue
	}

	return true
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

func startHTTPServer(stopChan chan struct{}, coll collector.Interface, logger *serviceLogger) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if coll != nil {
		if logger != nil {
			logger.Infof("Initializing metrics...")
		}
		if err := coll.RegisterMetrics(prometheus.DefaultRegisterer); err != nil {
			if logger != nil {
				logger.Errorf("Failed to register metrics: %v", err)
			}
		} else if err := coll.Start(ctx); err != nil {
			if logger != nil {
				logger.Errorf("Failed to start collector: %v", err)
			}
		} else {
			if logger != nil {
				logger.Infof("Metrics initialized")
			}
		}
	}

	go func() {
		<-stopChan
		cancel()
	}()
	// сервер с метриками
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		handshakeKey := r.Header.Get("X-Agent-Handshake-Key")
		if handshakeKey != secretkey {
			clientIP := r.RemoteAddr
			if logger != nil {
				logger.Warnf("Unauthorized request from IP: %s", clientIP)
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if logger != nil {
			logger.Infof("Received authorized request from IP: %s", r.RemoteAddr)
		}
		promhttp.Handler().ServeHTTP(w, r)
	})

	// сервер api
	apiMux := http.NewServeMux()

	// Создаем экземпляр обработчика
	uuidHandler := &api.UUIDHandler{}

	// Оборачиваем обработчики в middleware
	updateUUIDHandler := api.AuthMiddleware(http.HandlerFunc(uuidHandler.UpdateUUID))

	// Регистрируем обработчики в apiMux
	apiMux.Handle("/api/update-uuid", updateUUIDHandler)

	// Запуск серверов
	//server := &http.Server{Addr: ":9182"}
	metricsServer := &http.Server{Addr: ":9182", Handler: nil} // Default handler
	apiServer := &http.Server{Addr: ":9183", Handler: apiMux}

	go func() {
		if logger != nil {
			logger.Printf("Starting metrics server on :9182")
		}
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			if logger != nil {
				logger.Errorf("Metrics server error: %v", err)
			}
		}
	}()

	go func() {
		if logger != nil {
			logger.Printf("Starting API server on :9183")
		}
		certPath := "configs/certs"
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			// Если директория не существует, попробуем использовать ProgramData
			certPath = filepath.Join(os.Getenv("ProgramData"), "NITRINOnetControlManager", "certs")
		}
		if err := apiServer.ListenAndServeTLS(
			filepath.Join(certPath, "cert.pem"),
			filepath.Join(certPath, "key.pem"),
		); err != nil && err != http.ErrServerClosed {
			if logger != nil {
				logger.Errorf("API server error: %v", err)
			}
		}
	}()

	// Обработка остановки
	<-stopChan
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		if logger != nil {
			logger.Errorf("Metrics server shutdown error: %v", err)
		}
	}
	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		if logger != nil {
			logger.Errorf("API server shutdown error: %v", err)
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
