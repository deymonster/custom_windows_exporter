package watcher

import (
	"log"
	"node_exporter_custom/metrics"

	"github.com/fsnotify/fsnotify"
)

func WatchConfigFile(configFilePath string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	done := make(chan bool)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("Config file changed, reloading...")
					deviceConfig, err := metrics.ReadDeviceConfig()
					if err != nil {
						log.Printf("Error reading device config: %v", err)
					} else {
						metrics.UpdateSerialNumberMetrics(deviceConfig)
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Error watching config file: %v", err)
			}
		}
	}()

	err = watcher.Add(configFilePath)
	if err != nil {
		log.Fatalf("Failed to watch config file: %v", err)
	}

	<-done
}
