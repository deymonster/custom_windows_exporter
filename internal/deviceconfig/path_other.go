//go:build !windows && !linux

package deviceconfig

func defaultConfigPath() string {
	return "config.yml"
}
