package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"node_exporter_custom/internal/api"
	"node_exporter_custom/internal/collector"
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

	apiMux := http.NewServeMux()
	uuidHandler := &api.UUIDHandler{}
	updateUUIDHandler := api.AuthMiddleware(http.HandlerFunc(uuidHandler.UpdateUUID))
	apiMux.Handle("/api/update-uuid", updateUUIDHandler)

	metricsServer := &http.Server{Addr: ":9182", Handler: nil}
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

		certPath := os.Getenv("NCM_CERT_DIR")
		if certPath == "" {
			certPath = "configs/certs"
			if _, err := os.Stat(certPath); os.IsNotExist(err) {
				programData := os.Getenv("ProgramData")
				candidate := filepath.Join(programData, "NITRINOnetControlManager", "certs")
				if programData != "" {
					certPath = candidate
				}
			}
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
