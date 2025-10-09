//go:build windows

package metrics

import (
	"fmt"
	"log"
	"time"

	"github.com/StackExchange/wmi"
)

type Win32_BIOS struct {
	Manufacturer string
	Version      string
	ReleaseDate  string
}

func parseWMIDate(wmiDate string) (string, error) {
	if len(wmiDate) < 8 {
		return "", fmt.Errorf("invalid date format")
	}
	parseDate, err := time.Parse("20060102", wmiDate[:8])
	if err != nil {
		return "", err
	}
	return parseDate.Format("2006-01-02"), nil
}

func GetBiosInfo() ([]Win32_BIOS, error) {
	var bios []Win32_BIOS
	err := wmi.Query("SELECT Manufacturer, Version, ReleaseDate FROM Win32_BIOS", &bios)
	if err != nil {
		return nil, err
	}
	return bios, nil
}

func RecordBiosInfo() {
	biosData, err := GetBiosInfo()
	if err != nil || len(biosData) == 0 {
		log.Printf("Error getting bios info: %v", err)
		return
	}
	bios := biosData[0]
	formattedDate, err := parseWMIDate(bios.ReleaseDate)
	if err != nil {
		log.Printf("Error parsing bios release date: %v", err)
		return
	}

	BiosInfo.With(prometheus.Labels{
		"manufacturer": bios.Manufacturer,
		"version":      bios.Version,
		"release_date": formattedDate,
	}).Set(1)
}
