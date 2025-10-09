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

func SetupLogging() (io.Closer, error) {
	writer, err := newRotatingFileWriter(LogFilePath, 50*1024*1024, 5)
	if err != nil {
		return nil, err
	}

	multiWriter := io.MultiWriter(os.Stdout, writer)
	log.SetOutput(multiWriter)

	return writer, nil
}

func WriteLog(message string) {
	log.Println(message)
}

func CloseLog(logCloser io.Closer) {
	if logCloser != nil {
		logCloser.Close()
	}
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
