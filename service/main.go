//go:build windows

package main

import (
	"context"
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
	"node_exporter_custom/internal/secrets"
	"node_exporter_custom/logmanager"
)

type myService struct {
	collector collector.Interface
	logger    *serviceLogger
	secrets   *secrets.Manager

	mu        sync.Mutex
	cancel    context.CancelFunc
	serverErr chan error
}

func (m *myService) startServers() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.collector == nil {
		if m.logger != nil {
			m.logger.Errorf("collector not configured")
		}
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.serverErr = make(chan error, 1)

	if m.secrets != nil {
		go m.secrets.WatchFiles(ctx.Done(), m.logger.standardLogger())
	}

	go func() {
		err := startHTTPServer(ctx, m.collector, m.logger, m.secrets)
		m.serverErr <- err
	}()
}

func (m *myService) stopServers() {
	m.mu.Lock()
	cancel := m.cancel
	errCh := m.serverErr
	m.cancel = nil
	m.serverErr = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if errCh != nil {
		select {
		case err := <-errCh:
			if err != nil && m.logger != nil {
				m.logger.Errorf("server exited with error: %v", err)
			}
		case <-time.After(10 * time.Second):
			if m.logger != nil {
				m.logger.Warnf("server did not stop within timeout")
			}
		}
	}
}

func (m *myService) Execute(args []string, req <-chan svc.ChangeRequest, changes chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	changes <- svc.Status{State: svc.StartPending}

	if m.logger != nil {
		m.logger.Infof("service starting")
	}

	m.startServers()

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case c := <-req:
			switch c.Cmd {
			case svc.Stop, svc.Shutdown:
				if m.logger != nil {
					m.logger.Infof("service stopping")
				}
				m.stopServers()
				changes <- svc.Status{State: svc.StopPending}
				return
			case svc.Interrogate:
				changes <- c.CurrentStatus
			default:
				if m.logger != nil {
					m.logger.Warnf("unknown control request: %v", c.Cmd)
				}
			}
		case err := <-m.serverErr:
			if err != nil && m.logger != nil {
				m.logger.Errorf("server error: %v", err)
			}
			changes <- svc.Status{State: svc.StopPending}
			return
		}
	}
}

func runService(name string, isService bool, handler *myService) {
	if !isService {
		if handler.logger != nil {
			handler.logger.Infof("running in interactive mode")
		}
		if err := debug.Run(name, handler); err != nil {
			if handler.logger != nil {
				handler.logger.Errorf("debug run failed: %v", err)
			}
			os.Exit(1)
		}
		return
	}

	if err := svc.Run(name, handler); err != nil {
		if handler.logger != nil {
			handler.logger.Errorf("service run failed: %v", err)
		}
		os.Exit(1)
	}
}

func main() {
	logOptions := logmanager.DefaultOptions()
	logOptions.EnableFile = logOptions.FilePath != "" && shouldEnableFileLogging()

	logMgr, err := logmanager.New(logOptions)
	if err != nil {
		log.Fatalf("failed to setup logging: %v", err)
	}
	defer func() {
		if err := logMgr.Close(); err != nil {
			log.Printf("failed to close log manager: %v", err)
		}
	}()

	svcLogger := newServiceLogger(logMgr.Logger())
	if !logOptions.EnableFile {
		svcLogger.Printf("file logging disabled; stdout/stderr will capture service output")
	}

	collectorImpl, err := collector.New(runtime.GOOS)
	if err != nil {
		if svcLogger != nil {
			svcLogger.Errorf("failed to initialize collector for %s: %v", runtime.GOOS, err)
		}
		os.Exit(1)
	}

	secretsMgr := secrets.NewManager()
	if err := secretsMgr.Reload(); err != nil {
		if svcLogger != nil {
			svcLogger.Warnf("failed to load secrets: %v", err)
		}
	}
	defer secretsMgr.Close()

	serviceHandler := &myService{
		collector: collectorImpl,
		logger:    svcLogger,
		secrets:   secretsMgr,
	}

	isService, err := svc.IsWindowsService()
	if err != nil {
		if svcLogger != nil {
			svcLogger.Errorf("failed to determine service mode: %v", err)
		}
		os.Exit(1)
	}

	if !isService {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		go runService("NITRINOnetControlManager", false, serviceHandler)

		<-sigChan

		if svcLogger != nil {
			svcLogger.Infof("stopping interactive service")
		}

		serviceHandler.stopServers()
		return
	}

	runService("NITRINOnetControlManager", true, serviceHandler)
}
