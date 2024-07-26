package config

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the application configuration
type Config struct {
	GoogleCloudProjectID string `yaml:"googleCloudProjectId"`
	GKEClusterName       string `yaml:"gkeClusterName"`
	BadgerPath           string `yaml:"badgerPath"`
	KubernetesConfigPath string `yaml:"kubernetesConfigPath,omitempty"`
	Notification         struct {
		Slack []struct {
			Filter     string `yaml:"filter,omitempty"`
			WebhookURL string `yaml:"webhookUrl"`
		} `yaml:"slack"`
	} `yaml:"notification"`
}

// ParseFile parses configuration from a given file path
func ParseFile(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	return Parse(f)
}

// Parse parses configuration by reading the contents of the supplied reader
func Parse(r io.Reader) (Config, error) {
	var config Config
	if err := yaml.NewDecoder(r).Decode(&config); err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}
	return config, nil
}
