package config

import (
	"log"
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Version string      `yaml:"version"`
	Project ProjectInfo `yaml:"project"`
}

type ProjectInfo struct {
	GoMod      string            `yaml:"go.mod"`
	Entrypoint []string          `yaml:"entrypoint"`
	Ignore     []string          `yaml:"ignore"`
	Hooks      map[string]string `yaml:"hooks"`
}

func parseConfig(b []byte) (*Config, error) {
	var config Config
	if err := yaml.Unmarshal(b, &config); err != nil {
		return nil, err
	}
	for k, v := range config.Project.Hooks {
		log.Printf("%s: %s", k, v)
	}
	return &config, nil
}

// New returns a project config. path is veronica config file path.
// Now only support yaml.
func New(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to read %s", path)
	}
	cfg, err := parseConfig(content)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to parse %s", path)
	}
	return cfg, nil
}
