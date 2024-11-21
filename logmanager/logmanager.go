package logmanager

import (
	"log"
	"fmt"
	"io"
	"os"
)

const LogFilePath = `C:\ProgramData\NITRINOnetControlManager\service.log`

func CreateLogFile() error {
	if _, err := os.Stat(LogFilePath); os.IsNotExist(err) {
		file, err := os.Create(LogFilePath)
		if err != nil {
			return fmt.Errorf("failed to create log file: %v", err)
		}
		defer file.Close()
	}
	return nil
}

func SetupLogging() (*os.File, error) {
	err := CreateLogFile()
	if err !=nil {
		return nil, err
	}

	logFile, err := os.OpenFile(LogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)

	return logFile, nil
}

func WriteLog(message string) {
	log.Println(message)
}

func CloseLog(logFile *os.File) {

	if logFile != nil {
		logFile.Sync()
		logFile.Close()
	}
}