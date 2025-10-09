//go:build linux

package registryutil

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	uuidFileName = "hardware_uuid"
)

func stateDir() string {
	if dir := os.Getenv("NCM_STATE_DIR"); dir != "" {
		return dir
	}
	return "/var/lib/nitrinonetcmanager"
}

func uuidPath() string {
	return filepath.Join(stateDir(), uuidFileName)
}

func ReadUUIDFromRegistry() (string, error) {
	data, err := os.ReadFile(uuidPath())
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func WriteUUIDToRegistry(uuid string) error {
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(uuidPath(), []byte(uuid), 0o644)
}

func KeyExists() (bool, error) {
	if _, err := os.Stat(uuidPath()); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func CreateKey() error {
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(uuidPath()); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.WriteFile(uuidPath(), []byte(""), 0o644)
		}
		return err
	}
	return nil
}
