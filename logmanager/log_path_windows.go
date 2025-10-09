//go:build windows

package logmanager

func defaultLogFilePath() string {
	return `C:\\ProgramData\\NITRINOnetControlManager\\service.log`
}
