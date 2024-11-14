package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

func WatchAll(config *Config, force bool, stopAll chan struct{}) error {
	sources, err := getSources(config)
	if err != nil {
		return err
	}

	stopChans := make(map[string]chan struct{})

	go func() {
		<-stopAll
		for _, stop := range stopChans {
			stop <- struct{}{}
		}
	}()

	wg := sync.WaitGroup{}

	setErr := func(e error) {
		err = e
	}

	for _, source := range sources {
		wg.Add(1)
		stop := make(chan struct{})
		stopChans[source] = stop
		go func() {
			defer wg.Done()

			err = Watch(source, config, force, stop)
			if err != nil {
				stopAll <- struct{}{}
				setErr(err)
			}
		}()
	}

	wg.Wait()

	return err
}

func Watch(path string, config *Config, force bool, stop chan struct{}) error {
	errChan := make(chan error)
	stopErrorHandlingChan := make(chan struct{})

	go func() {
		for {
			select {
			case err := <-errChan:
				// TODO: handle compilation errors
				fmt.Println(err)
			case <-stopErrorHandlingChan:
				return
			}
		}
	}()

	defer func() {
		stopErrorHandlingChan <- struct{}{}
	}()

	// Make sure the file has been built at least once
	// FIXME: What happens if the file changes while it is getting built?
	err := BuildFile(path, config, 1, force, stop)
	if err != nil {
		errChan <- err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	err = watcher.Add(path)
	if err != nil {
		return err
	}
	defer func() {
		watcher.Close()
		fmt.Println("end", path)
	}()

	fmt.Printf("watching %s\n", path)

	writeId := 0

	for {
		select {
		case <-stop:
			return nil
		case event := <-watcher.Events:
			fmt.Println(event)
			if event.Has(fsnotify.Remove) {
				fmt.Fprintf(os.Stderr, "The file %s was removed, stopping the watcher\n", path)
				return nil
			}

			if event.Has(fsnotify.Rename) && event.Name != path {
				fmt.Fprintf(os.Stderr, "The file %s was renamed, stopping the watcher\n", path)
				// TODO: should we add a new watcher here?
			}

			if event.Has(fsnotify.Write) || (event.Has(fsnotify.Rename) && event.Name == path) {
				writeId++
				go func(currentWriteId int) {
					fmt.Fprintf(os.Stderr, "%s has changed\n", path)
					// Wait some time for the write to finish, not very elegant I know
					<-time.After(200 * time.Millisecond)
					if currentWriteId == writeId {
						err := BuildFile(path, config, 1, force, stop)
						if err != nil {
							errChan <- err
						}
					}
				}(writeId)
			}

		case err = <-watcher.Errors:
			// TODO: do we actually want to return here?
			return err
		}

		fmt.Println("loopy")
	}
}
