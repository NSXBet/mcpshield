package pkg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	API        APIConfig         `yaml:"api"`
	Auth       AuthConfig        `yaml:"auth"`
	Log        LogConfig         `yaml:"log"`
	Server     ServerConfig      `yaml:"server"`
	Runtime    RuntimeConfig     `yaml:"runtime"`
	MCPServers []MCPServerConfig `yaml:"mcp-servers"`
}

type APIConfig struct {
	Endpoint string `yaml:"endpoint"`
	Version  string `yaml:"version"`
	Timeout  int    `yaml:"timeout"`
}

type AuthConfig struct {
	Timeout int `yaml:"timeout"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Color  bool   `yaml:"color"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type MCPServerConfig struct {
	Name    string            `yaml:"name"`
	Image   string            `yaml:"image"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env,omitempty"`
}

type KubernetesConfig struct {
	Namespace  string `yaml:"namespace"`
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
}

type RuntimeConfig struct {
	Kubernetes *KubernetesConfig `yaml:"kubernetes"`
}

// Config accessor methods
func (c *Config) GetKubernetesNamespace() string {
	if c.Runtime.Kubernetes == nil {
		return "default"
	}
	return c.Runtime.Kubernetes.Namespace
}

func (c *Config) HasKubernetesRuntime() bool {
	return c.Runtime.Kubernetes != nil
}

func (c *Config) GetKubeconfig() string {
	if c.Runtime.Kubernetes == nil || c.Runtime.Kubernetes.Kubeconfig == "" {
		return ""
	}
	return c.Runtime.Kubernetes.Kubeconfig
}

func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func (c *Config) GetLogLevel() string {
	return c.Log.Level
}

func (c *Config) GetMCPServers() []MCPServerConfig {
	return c.MCPServers
}

func ReadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &config, nil
} 