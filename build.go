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
	"time"
)

func getOutputPath(path string, baseDir string) string {
	basePath := filepath.Base(path)
	return filepath.Join(baseDir, basePath[:len(basePath)-4]+".pdf")
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

func BuildFile(job *Job, config *Config, recursion int) error {
	startTime := time.Now()
	err := ensureOutDirectories(config)
	if err != nil {
		return err
	}

	fmt.Printf("Builder: %s %s\n",
		func() string {
			if recursion == 1 {
				return "Building"
			} else {
				return "Rebuiling"
			}
		}(),
		job.path,
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	stopped := false

	go func() {
		select {
		case <-job.stop:
			fmt.Printf("Builder: %s cancelled\n", job.path)
			stopped = true
			cancel()
		case <-done:
			return
		}
	}()

	cmd := exec.CommandContext(
		ctx,
		config.Compiler,
		"--output-directory="+config.AuxDir,
		"--interaction=nonstopmode",
		func() string {
			if config.ShellEscape {
				return "--shell-escape"
			} else {
				return "--no-shell-escape"
			}
		}(),
		job.path,
	)

	output, err := cmd.CombinedOutput()
	if stopped {
		fmt.Printf("Builder: job %s cancelled\n", job.path)
		return nil
	}

	done <- struct{}{}
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			// fmt.Printf("Builder: build of %s failed with code: %d\n", job.path, exitErr.ProcessState.ExitCode())
			_, _ = os.Stderr.Write(output)
			return exitErr
		}
		return err
	}

	fmt.Printf("Builder: completed compiling %s for the %s time in %fs\n",
		job.path,
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
			return fmt.Errorf("Maximum recursion reached for %s", job.path)
		}

		return BuildFile(job, config, recursion+1)
	}

	err = os.Rename(
		getOutputPath(job.path, config.AuxDir),
		getOutputPath(job.path, config.OutputFolder),
	)

	return err
}

func BuildAll(config *Config, force bool, stopAll chan struct{}) error {
	startTime := time.Now()
	sources, err := getSources(config)
	if err != nil {
		return err
	}

	jobDispatcher := NewJobDispatcher(config, stopAll)

	jobDispatcher.Start()

	for _, source := range sources {
		err = jobDispatcher.AddJob(source, force)
		if err != nil {
			return err
		}
	}

	jobDispatcher.Wait()

	fmt.Printf("Built %d files in %fs\n",
		len(sources),
		time.Since(startTime).Seconds(),
	)

	return nil
}
