//go:build linux

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"node_exporter_custom/internal/collector"
	"node_exporter_custom/internal/secrets"
	"node_exporter_custom/logmanager"
)

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
	if !logOptions.EnableFile && svcLogger != nil {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secretsMgr.WatchFiles(ctx.Done(), svcLogger.standardLogger())

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- startHTTPServer(ctx, collectorImpl, svcLogger, secretsMgr)
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	for {
		select {
		case sig := <-signalChan:
			switch sig {
			case syscall.SIGHUP:
				if err := secretsMgr.Reload(); err != nil {
					if svcLogger != nil {
						svcLogger.Errorf("failed to reload secrets: %v", err)
					}
				} else {
					secretsMgr.WatchFiles(ctx.Done(), svcLogger.standardLogger())
					if svcLogger != nil {
						svcLogger.Infof("reloaded secrets from environment")
					}
				}
			default:
				if svcLogger != nil {
					svcLogger.Infof("received signal: %s, shutting down", sig)
				}
				cancel()
				if err := <-serverErr; err != nil {
					if svcLogger != nil {
						svcLogger.Errorf("server exited with error: %v", err)
					}
					os.Exit(1)
				}
				if svcLogger != nil {
					svcLogger.Infof("service stopped")
				}
				return
			}
		case err := <-serverErr:
			if err != nil {
				if svcLogger != nil {
					svcLogger.Errorf("server exited with error: %v", err)
				}
				os.Exit(1)
			}
			if svcLogger != nil {
				svcLogger.Infof("service stopped")
			}
			return
		}
	}
}
