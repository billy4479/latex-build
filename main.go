package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	err := (&cli.App{
		Name: "latex-build",
		Action: func(ctx *cli.Context) error {
			config, err := LoadConfig()
			if err != nil {
				return err
			}

			if ctx.Bool("watch") {
				return Watch(config)
			} else {
				return BuildAll(config)
			}
		},
		Commands: []*cli.Command{
			{
				Name: "init",
				Action: func(ctx *cli.Context) error {
					config := NewConfig()
					return WriteConfig(config)
				},
			},
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:     "watch",
				Required: false,
				Value:    false,
				Aliases:  []string{"w"},
			},
		},
	}).Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
