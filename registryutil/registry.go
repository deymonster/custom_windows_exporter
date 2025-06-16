package registryutil

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const (
	regPath  = `SOFTWARE\NITRINOnetControlManager\Agent`
	valueKey = "HardwareUUID"
)

// ReadUUIDFromRegistry reads the unique hardware UUID from the registry
// under LOCAL_MACHINE\SOFTWARE\NITRINOnetControlManager\Agent\HardwareUUID
// and returns it as a string. If the value does not exist or reading fails,
// it returns an error.
func ReadUUIDFromRegistry() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, regPath, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()

	uuid, _, err := key.GetStringValue(valueKey)
	if err != nil {
		return "", fmt.Errorf("failed to read UUID from registry: %v", err)
	}
	return uuid, nil
}

// WriteUUIDToRegistry writes the unique hardware UUID to the registry under
// LOCAL_MACHINE\SOFTWARE\NITRINOnetControlManager\Agent\HardwareUUID. If the
// value does not exist or writing fails, it returns an error.
func WriteUUIDToRegistry(uuid string) error {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, regPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	return key.SetStringValue(valueKey, uuid)
}


// KeyExists checks if the registry key under LOCAL_MACHINE\SOFTWARE\NITRINOnetControlManager\Agent
// exists and returns a boolean value indicating its existence. If the key does not exist or there
// is an error while checking, it returns an error.
func KeyExists() (bool, error) {
    key, err := registry.OpenKey(registry.LOCAL_MACHINE, regPath, registry.QUERY_VALUE)
    if err != nil {
        if err == registry.ErrNotExist {
            return false, nil
        }
        return false, err
    }
    key.Close()
    return true, nil
}


// CreateKey creates a registry key under LOCAL_MACHINE\SOFTWARE\NITRINOnetControlManager\Agent with
// full access. If the key already exists or there is an error while creating, it returns an error.
func CreateKey() error {
    _, _, err := registry.CreateKey(registry.LOCAL_MACHINE, regPath, registry.ALL_ACCESS)
    return err
}

