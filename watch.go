package main

import (
	"fmt"
	"slices"
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
				fmt.Printf("Watcher: got error %v\n", err)

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
		err = watcher.Add(file)
		if err != nil {
			return err
		}
		fmt.Printf("Watcher: watching %s\n", file)

		// Make sure the file has been built at least once
		err := jobDispatcher.AddJob(file, force)
		if err != nil {
			return err
		}
	}

	for {
		select {
		case <-stopAll:
			return nil
		case event := <-watcher.Events:
			fmt.Println(event)
			if event.Has(fsnotify.Remove) {
				fmt.Printf("Watcher: %s was removed\n", event.Name)
				continue
			}

			if event.Has(fsnotify.Rename) &&
				// This is because vim doesn't write to them directly.
				// If it was a file we were already watching continue as if it was a write.
				!slices.Contains(files, event.Name) {
				// The file was renamed to something new
				// TODO: Should we add a new watcher here? Cancel the existing one?

				fmt.Printf("Watcher: %s was renamed\n", event.Name)
				continue
			}

			if event.Has(fsnotify.Write) || (event.Has(fsnotify.Rename) && slices.Contains(files, event.Name)) {
				go func() {
					fmt.Printf("Watcher: %s has changed\n", event.Name)
					// Wait some time for the write to finish, not very elegant I know
					<-time.After(200 * time.Millisecond)
					fmt.Printf("Watcher: Scheduling rebuild for %s\n", event.Name)
					err := jobDispatcher.AddJob(event.Name, force)
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
