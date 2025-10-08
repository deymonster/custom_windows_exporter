package deviceconfig

import (
	"context"
	"log"

	"github.com/fsnotify/fsnotify"
)

func Watch(ctx context.Context, path string, onChange func(*Config)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					config, err := Read(path)
					if err != nil {
						log.Printf("Error reading device config: %v", err)
						continue
					}
					onChange(config)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Error watching config file: %v", err)
			}
		}
	}()

	if err := watcher.Add(path); err != nil {
		watcher.Close()
		return err
	}

	return nil
}
