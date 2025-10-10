//go:build windows

package deviceconfig

func defaultConfigPath() string {
	return `C:\\ProgramData\\NITRINOnetControlManager\\config.yml`
}
