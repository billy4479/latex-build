package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

func WatchAll(config *Config, force bool, stopAll chan struct{}) error {
	sources, err := getSources(config)
	if err != nil {
		return err
	}

	return Watch(sources, config, force, stopAll)
}

func Watch(files []string, config *Config, force bool, stopAll chan struct{}) error {
	jobDispatcher := NewJobDispatcher(config, stopAll)
	jobDispatcher.Start()

	errors := make(chan error)

	go func() {
		for {
			select {
			case err := <-errors:
				fmt.Printf("[Watcher]: got error %v\n", err)

			case <-stopAll:
				return
			}
		}
	}()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	for _, file := range files {
		fmt.Printf("[Watcher]: watching %s\n", file)
		// err = watcher.Add(file)
		// if err != nil {
		// 	return err
		// }

		parent := filepath.Dir(file)
		// fmt.Printf("[Watcher]: watching %s\n", parent)
		err = watcher.Add(parent)
		if err != nil {
			return err
		}
		// Make sure the file has been built at least once
		err := jobDispatcher.AddJob(file, force)
		if err != nil {
			return err
		}
	}

	toBeRebuilt := make(map[string]int)
	toBeRebuiltLock := sync.Mutex{}

	for {
		select {
		case <-stopAll:
			jobDispatcher.Wait()
			return nil
		case event := <-watcher.Events:
			file := filepath.Clean(event.Name)
			if !slices.Contains(files, file) {
				continue
			}

			if event.Has(fsnotify.Write) {
				go func() {
					fmt.Printf("[Watcher]: %s has changed\n", file)

					toBeRebuiltLock.Lock()
					toBeRebuilt[file] += 1
					myId := toBeRebuilt[file]
					toBeRebuiltLock.Unlock()

					// Wait some time for the write to finish and for the user to finish typing
					<-time.After(200 * time.Millisecond)

					toBeRebuiltLock.Lock()
					if myId != toBeRebuilt[file] {
						toBeRebuiltLock.Unlock()
						fmt.Printf("[Watcher]: Dropping build for %s for event %s:%d\n", file, event.Op.String(), myId)
						return
					}
					toBeRebuiltLock.Unlock()

					fmt.Printf("[Watcher]: Scheduling rebuild for %s because of %s:%d\n", file, event.Op.String(), myId)
					err := jobDispatcher.AddJob(file, force)
					if err != nil {
						errors <- err
						return
					}
				}()
			}

		case err = <-watcher.Errors:
			// TODO: do we actually want to return here?
			return err
		}
	}
}
