//go:build windows

package main

import (
	"log"

	"golang.org/x/sys/windows/svc/eventlog"
)

func main() {
	err := eventlog.Remove("NITRINOnetControlManager")
	if err != nil {
		log.Printf("Failed to remove logger: %v", err)
	} else {
		log.Println("Event source removed")
	}
}
