package pkg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type MCPConfig struct {
	MCPServers []MCPServerConfig `yaml:"mcp-servers"`
	Runtime    RuntimeConfig     `yaml:"runtime"`
}

type MCPServerConfig struct {
	Name    string            `yaml:"name"`
	Image   string            `yaml:"image"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env,omitempty"`
}

type KubernetesConfig struct {
	Namespace string `yaml:"namespace"`
}

type RuntimeConfig struct {
	Kubernetes *KubernetesConfig `yaml:"kubernetes"`
}

func ReadConfig(filename string) (*MCPConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var config MCPConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &config, nil
} 