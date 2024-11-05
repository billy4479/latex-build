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

func BuildFile(path string, config *Config, recursion int) error {
	err := ensureOutDirectories(config)
	if err != nil {
		return err
	}

	ok, err := needsBuild(path, config)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	cancelMapLock.Lock()
	cancelMap[path] = cancel
	cancelMapLock.Unlock()

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

	defer func() {
		cancelMapLock.Lock()
		delete(cancelMap, path)
		cancelMapLock.Unlock()
	}()

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil
		}

		exitErr, ok := err.(*exec.ExitError)
		if ok {
			fmt.Fprintf(os.Stderr, "Build failed with code: %d\n", exitErr.ProcessState.ExitCode())
			_, _ = os.Stderr.Write(output)
			return exitErr
		}
		return err
	}

	if strings.Contains(string(output), "undefined references") {
		// TODO: I don't care about bibtex because I don't use it,
		//       but I guess this is where I'd add support to it

		// I think we should build at most twice, right?
		if recursion >= 2 {
			return fmt.Errorf("Maximum recursion reached")
		}
		return BuildFile(path, config, recursion+1)
	}

	err = os.Rename(
		getOutputPath(path, config.AuxDir),
		getOutputPath(path, config.OutputFolder),
	)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "%s\n", path)

	return nil
}

func BuildAll(config *Config) error {
	sources, err := getSources(config)
	if err != nil {
		return err
	}

	hasError := false
	wg := sync.WaitGroup{}
	for _, source := range sources {
		wg.Add(1)
		go func(source string) {
			err = BuildFile(source, config, 1)
			if err != nil {
				fmt.Fprintf(os.Stderr, "build %s: %v\n", source, err)
				hasError = true
			}
			wg.Done()
		}(source)
	}
	wg.Wait()

	if hasError {
		return fmt.Errorf("Compilation completed with errors")
	}
	return nil
}

func Watch(config *Config) error {

	return nil
}
