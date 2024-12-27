package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/urfave/cli/v2"
)

func main() {
	err := (&cli.App{
		Name: "latex-build",
		Authors: []*cli.Author{
			{Name: "billy4479"},
		},
		Usage: "Build and watch latex files",
		Action: func(ctx *cli.Context) error {
			config, err := LoadConfig()
			if err != nil {
				return err
			}
			force := ctx.Bool("force")

			sigintChan := make(chan os.Signal, 1)
			signal.Notify(sigintChan, os.Interrupt)
			stopAll := make(chan struct{})

			go func() {
				<-sigintChan
				fmt.Println("\nstopping")
				close(stopAll)
			}()

			if ctx.Bool("watch") {
				return WatchAll(config, force, stopAll)
			} else {
				return BuildAll(config, force, stopAll)
			}

		},
		Commands: []*cli.Command{
			{
				Name: "init",
				Action: func(ctx *cli.Context) error {
					config := NewConfig()
					return WriteConfig(config)
				},
				Usage: "Create a new config",
			},
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:     "force",
				Required: false,
				Value:    false,
				Aliases:  []string{"f"},
				Usage:    "Force recompilation",
			},
			&cli.BoolFlag{
				Name:     "watch",
				Required: false,
				Value:    false,
				Aliases:  []string{"w"},
				Usage:    "Watch and rebuild the file(s) on changes",
			},
		},
	}).Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
