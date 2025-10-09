//go:build linux

package main

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"node_exporter_custom/internal/collector"
	"node_exporter_custom/logmanager"
)

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

	stopChan := make(chan struct{})
	serverDone := make(chan struct{})

	go func() {
		startHTTPServer(stopChan, collectorImpl, svcLogger)
		close(serverDone)
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	sig := <-signalChan
	if svcLogger != nil {
		svcLogger.Infof("Received signal: %s, shutting down", sig)
	}

	close(stopChan)
	<-serverDone

	if svcLogger != nil {
		svcLogger.Infof("Service stopped.")
	}
}
