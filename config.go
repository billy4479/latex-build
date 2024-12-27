package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Compiler        string   `yaml:"compiler"`
	OutputFolder    string   `yaml:"output-folder"`
	AuxDir          string   `yaml:"aux-dir"`
	ShellEscape     bool     `yaml:"shell-escape"`
	AdditionalFlags []string `yaml:"additional-flags"`
	IncludeFiles    []string `yaml:"include-files"`
	Parallel        int      `yaml:"parallel"`
}

func NewConfig() *Config {
	return &Config{
		Compiler:        "lualatex",
		OutputFolder:    "build",
		AuxDir:          "build/aux",
		ShellEscape:     false,
		AdditionalFlags: []string{},
		IncludeFiles:    []string{"*.tex"},
		Parallel:        0,
	}
}

var configPath = "latex-build.yaml"

func LoadConfig() (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}

	defer file.Close()
	config := NewConfig()
	err = yaml.NewDecoder(file).Decode(&config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func WriteConfig(config *Config) error {
	file, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	err = encoder.Encode(config)
	return err
}
