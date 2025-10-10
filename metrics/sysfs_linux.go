//go:build linux

package metrics

import (
	"os"
	"strings"
)

func readSysfsValue(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
