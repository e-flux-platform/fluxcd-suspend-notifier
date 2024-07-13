package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

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

func Parse(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	var config Config
	if err = yaml.NewDecoder(f).Decode(&config); err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}
	return config, nil
}
