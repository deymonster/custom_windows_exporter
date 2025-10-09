//go:build !windows

package logmanager

func defaultLogFilePath() string {
	return "/var/log/nitrinonetcmanager/service.log"
}
