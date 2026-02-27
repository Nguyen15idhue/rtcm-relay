package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Source      SourceConfig      `yaml:"source"`
	Destination DestinationConfig `yaml:"destination"`
	Logging     LoggingConfig     `yaml:"logging"`
}

type SourceConfig struct {
	Interface string `yaml:"interface"`
	Port      int    `yaml:"port"`
}

type DestinationConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
