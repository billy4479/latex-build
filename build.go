package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func getOutputPath(path string, baseDir string) string {
	return filepath.Join(baseDir, path[:len(path)-4]+".pdf")
}

func needsBuild(path string, config *Config) (bool, error) {

	infoSource, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	pdfPath := getOutputPath(path, config.OutputFolder)
	infoPdf, err := os.Stat(pdfPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, nil
		} else {
			return false, err
		}
	}

	return infoSource.ModTime().After(infoPdf.ModTime()), nil
}

func getSources(config *Config) ([]string, error) {
	sources := []string{}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		for _, pattern := range config.IncludeFiles {
			matched, err := filepath.Match(pattern, path)
			if err != nil {
				return err
			}

			if matched {
				sources = append(sources, path)
				return nil
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return sources, nil
}

func ensureOutDirectories(config *Config) error {
	err := os.MkdirAll(config.OutputFolder, 0755)
	if err != nil {
		return err
	}
	if config.AuxDir != "" {
		err = os.MkdirAll(config.AuxDir, 0755)
		if err != nil {
			return err
		}
	}
	return nil
}

var (
	cancelMap     map[string]context.CancelFunc = make(map[string]context.CancelFunc)
	cancelMapLock sync.Mutex
)

func BuildFile(path string, config *Config, recursion int, force bool, stop chan struct{}) error {
	startTime := time.Now()
	err := ensureOutDirectories(config)
	if err != nil {
		return err
	}

	if !force {
		ok, err := needsBuild(path, config)
		if err != nil {
			return err
		}

		if !ok {
			return nil
		}
	}

	fmt.Printf("%s %s\n",
		func() string {
			if recursion == 1 {
				return "Building"
			} else {
				return "Rebuiling"
			}
		}(),
		path,
	)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel old jobs before starting a new one
	cancelMapLock.Lock()
	if oldCancel, ok := cancelMap[path]; ok {
		oldCancel()
	}
	cancelMap[path] = cancel
	cancelMapLock.Unlock()

	defer func() {
		cancelMapLock.Lock()
		delete(cancelMap, path)
		cancelMapLock.Unlock()
	}()

	go func() {
		<-stop
		cancel()
	}()

	cmd := exec.CommandContext(
		ctx,
		config.Compiler,
		"--output-directory="+config.AuxDir,
		"--interaction-mode=nonstop",
		func() string {
			if config.ShellEscape {
				return "--shell-escape"
			} else {
				return "--no-shell-escape"
			}
		}(),
		path,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil
		}

		exitErr, ok := err.(*exec.ExitError)
		if ok {
			fmt.Fprintf(os.Stderr, "Build of %s failed with code: %d\n", path, exitErr.ProcessState.ExitCode())
			_, _ = os.Stderr.Write(output)
			return exitErr
		}
		return err
	}

	fmt.Printf("Completed compiling %s for the %s time in %fs\n",
		path,
		func() string {
			if recursion == 1 {
				return "1st"
			} else {
				return "2nd"
			}
		}(),
		time.Since(startTime).Seconds(),
	)

	if strings.Contains(string(output), "undefined references") {
		// TODO: I don't care about bibtex because I don't use it,
		//       but I guess this is where I'd add support to it

		// I think we should build at most twice, right?
		if recursion >= 2 {
			return fmt.Errorf("Maximum recursion reached for %s", path)
		}

		return BuildFile(path, config, recursion+1, force, stop)
	}

	err = os.Rename(
		getOutputPath(path, config.AuxDir),
		getOutputPath(path, config.OutputFolder),
	)
	if err != nil {
		return err
	}

	return nil
}

func BuildAll(config *Config, force bool, stopAll chan struct{}) error {
	startTime := time.Now()
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

	hasError := false
	wg := sync.WaitGroup{}
	for _, source := range sources {
		wg.Add(1)
		stop := make(chan struct{})
		stopChans[source] = stop
		go func(source string) {
			err = BuildFile(source, config, 1, force, stop)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Build error %s: %v\n", source, err)
				hasError = true
			}
			wg.Done()
		}(source)
	}
	wg.Wait()

	fmt.Printf("Built %d files in %fs\n",
		len(sources),
		time.Since(startTime).Seconds(),
	)

	if hasError {
		return fmt.Errorf("Compilation completed with errors")
	}
	return nil
}
