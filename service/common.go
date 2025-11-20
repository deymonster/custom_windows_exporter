package main

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"runtime"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"node_exporter_custom/internal/api"
	"node_exporter_custom/internal/collector"
	"node_exporter_custom/internal/secrets"
)

type serviceLogger struct {
	logger *log.Logger
}

func newServiceLogger(base *log.Logger) *serviceLogger {
	if base == nil {
		base = log.Default()
	}
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
	l.log(format, args...)
}

func (l *serviceLogger) log(format string, args ...interface{}) {
	if l == nil || l.logger == nil {
		return
	}

	if len(args) == 0 {
		l.logger.Println(format)
		return
	}

	l.logger.Printf(format, args...)
}

func (l *serviceLogger) standardLogger() *log.Logger {
	if l == nil {
		return log.Default()
	}
	return l.logger
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

func startHTTPServer(ctx context.Context, coll collector.Interface, logger *serviceLogger, secretsMgr *secrets.Manager) error {
	if coll != nil {
		if logger != nil {
			logger.Infof("initializing metrics")
		}
		if err := coll.RegisterMetrics(prometheus.DefaultRegisterer); err != nil {
			if logger != nil {
				logger.Errorf("failed to register metrics: %v", err)
			}
			return fmt.Errorf("register metrics: %w", err)
		}
		if err := coll.Start(ctx); err != nil {
			if logger != nil {
				logger.Errorf("failed to start collector: %v", err)
			}
			return fmt.Errorf("start collector: %w", err)
		}
		if logger != nil {
			logger.Infof("metrics collectors started")
		}
	}

	if logger != nil {
		expected := ""
		if secretsMgr != nil {
			expected = strings.TrimSpace(secretsMgr.HandshakeKey())
		}
		if expected == "" {
			logger.Warnf("NCM_HANDSHAKE_KEY is not configured; metrics requests will be rejected")
		}
	}

	metricsHandler := promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{})
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		expected := ""
		if secretsMgr != nil {
			expected = strings.TrimSpace(secretsMgr.HandshakeKey())
		}
		provided := strings.TrimSpace(r.Header.Get("X-Agent-Handshake-Key"))

		if expected == "" {
			if logger != nil {
				logger.Warnf("handshake key not configured; rejecting metrics request from %s", r.RemoteAddr)
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
			if logger != nil {
				logger.Warnf("unauthorized metrics request from %s", r.RemoteAddr)
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		metricsHandler.ServeHTTP(w, r)
	})

	apiHandler := api.NewRouter(secretsMgr)

	metricsServer := &http.Server{Addr: ":9182", Handler: metricsMux}
	apiServer := &http.Server{Addr: ":9183", Handler: apiHandler}

	errCh := make(chan error, 2)

	go func() {
		if logger != nil {
			logger.Printf("starting metrics server on :9182")
		}
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("metrics server: %w", err)
		}
	}()

	go func() {
		if logger != nil {
			logger.Printf("starting API server on :9183")
		}

		certDir := resolveCertDir()
		certPath := filepath.Join(certDir, "cert.pem")
		keyPath := filepath.Join(certDir, "key.pem")

		if err := apiServer.ListenAndServeTLS(certPath, keyPath); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("api server: %w", err)
		}
	}()

	var serveErr error
	select {
	case <-ctx.Done():
	case err := <-errCh:
		serveErr = err
		if logger != nil {
			logger.Errorf("server error: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := metricsServer.Shutdown(shutdownCtx); err != nil && logger != nil {
		logger.Errorf("metrics server shutdown error: %v", err)
	}
	if err := apiServer.Shutdown(shutdownCtx); err != nil && logger != nil {
		logger.Errorf("api server shutdown error: %v", err)
	}

	if serveErr != nil {
		return serveErr
	}
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func resolveCertDir() string {
	if dir := strings.TrimSpace(os.Getenv("NCM_CERT_DIR")); dir != "" {
		return dir
	}
	if runtime.GOOS == "windows" {
		return `C:\ProgramData\NITRINOnetControlManager\certs`
	}
	return "configs/certs"
}
