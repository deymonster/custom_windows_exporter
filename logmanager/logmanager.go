package logmanager

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// For production
const LogFilePath = `C:\ProgramData\NITRINOnetControlManager\service.log`

// For testing
//const LogFilePath = `service_test.log`

// Options control how the logger is configured.
type Options struct {
	EnableStdout bool
	EnableFile   bool
	FilePath     string
	MaxSize      int64
	MaxBackups   int
	LoggerFlags  int
}

// DefaultOptions returns the default logging configuration used by the service.
func DefaultOptions() Options {
	return Options{
		EnableStdout: true,
		EnableFile:   true,
		FilePath:     LogFilePath,
		MaxSize:      50 * 1024 * 1024,
		MaxBackups:   5,
		LoggerFlags:  log.LstdFlags,
	}
}

// Manager wraps a configured logger and tracks any resources that need to be closed.
type Manager struct {
	logger  *log.Logger
	closers []io.Closer
	mu      sync.Mutex
}

// New creates a logging manager based on the provided options. The returned manager
// sets the global logger to share the same output and must be closed when the
// service shuts down.
func New(opts Options) (*Manager, error) {
	if opts.LoggerFlags == 0 {
		opts.LoggerFlags = log.LstdFlags
	}

	var writers []io.Writer
	var closers []io.Closer

	if opts.EnableStdout {
		writers = append(writers, os.Stdout)
	}

	if opts.EnableFile && opts.FilePath != "" {
		writer, err := newRotatingFileWriter(opts.FilePath, opts.MaxSize, opts.MaxBackups)
		if err != nil {
			return nil, err
		}
		writers = append(writers, writer)
		closers = append(closers, writer)
	}

	if len(writers) == 0 {
		writers = append(writers, io.Discard)
	}

	multiWriter := io.MultiWriter(writers...)
	log.SetOutput(multiWriter)
	log.SetFlags(opts.LoggerFlags)

	logger := log.New(multiWriter, "", opts.LoggerFlags)

	return &Manager{logger: logger, closers: closers}, nil
}

// Logger returns the configured logger instance.
func (m *Manager) Logger() *log.Logger {
	if m == nil {
		return log.Default()
	}
	return m.logger
}

// Close releases any resources held by the manager (for example, file handles).
func (m *Manager) Close() error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error
	for _, closer := range m.closers {
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	m.closers = nil
	return firstErr
}

type rotatingFileWriter struct {
	path       string
	maxSize    int64
	maxBackups int

	mu   sync.Mutex
	file *os.File
	size int64
}

func newRotatingFileWriter(path string, maxSize int64, maxBackups int) (*rotatingFileWriter, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("maxSize must be positive")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat log file: %v", err)
	}

	return &rotatingFileWriter{
		path:       path,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		file:       file,
		size:       info.Size(),
	}, nil
}

func (w *rotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return 0, errors.New("log writer is closed")
	}

	if w.size+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	err := w.file.Close()
	w.file = nil
	w.size = 0
	return err
}

func (w *rotatingFileWriter) rotate() error {
	if w.file == nil {
		return errors.New("log writer is closed")
	}

	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close log file before rotation: %v", err)
	}

	if w.maxBackups > 0 {
		oldest := fmt.Sprintf("%s.%d", w.path, w.maxBackups)
		if err := os.Remove(oldest); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove old log backup: %v", err)
		}

		for i := w.maxBackups - 1; i >= 1; i-- {
			oldName := fmt.Sprintf("%s.%d", w.path, i)
			newName := fmt.Sprintf("%s.%d", w.path, i+1)
			if err := os.Rename(oldName, newName); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("failed to rotate log %s to %s: %v", oldName, newName, err)
				}
			}
		}

		backupName := fmt.Sprintf("%s.1", w.path)
		if err := os.Rename(w.path, backupName); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("failed to create log backup: %v", err)
			}
		}
	} else {
		if err := os.Remove(w.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to remove log file during rotation: %v", err)
		}
	}

	newFile, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file after rotation: %v", err)
	}

	w.file = newFile
	w.size = 0
	return nil
}
