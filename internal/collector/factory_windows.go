//go:build windows

package collector

import "node_exporter_custom/internal/collector/windows"

func newForOS(_ string) (Interface, error) {
	return windows.New(), nil
}
