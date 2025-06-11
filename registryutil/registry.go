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

func WriteUUIDToRegistry(uuid string) error {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, regPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	return key.SetStringValue(valueKey, uuid)
}
