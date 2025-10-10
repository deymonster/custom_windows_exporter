//go:build linux

package collector

import "node_exporter_custom/internal/collector/linux"

func newForOS(_ string) (Interface, error) {
        return linux.New(), nil
}
