package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/michaelbrown/forge/internal/tools"
)

type ProviderConfig struct {
	BaseURL string            `mapstructure:"base_url"`
	APIKey  string            `mapstructure:"api_key"`
	Models  map[string]string `mapstructure:"models"`
}

type AgentConfig struct {
	MaxIterations int    `mapstructure:"max_iterations"`
	ProfilesDir   string `mapstructure:"profiles_dir"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type StorageConfig struct {
	DBPath string `mapstructure:"db_path"`
}

type Config struct {
	Providers       map[string]ProviderConfig        `mapstructure:"providers"`
	DefaultProvider string                           `mapstructure:"default_provider"`
	Agent           AgentConfig                      `mapstructure:"agent"`
	Server          ServerConfig                     `mapstructure:"server"`
	Storage         StorageConfig                    `mapstructure:"storage"`
	Tools           map[string]tools.ToolServerConfig `mapstructure:"tools"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("forge")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.forge")

	v.SetDefault("default_provider", "ollama")
	v.SetDefault("agent.max_iterations", 10)
	v.SetDefault("server.port", 8080)
	v.SetDefault("storage.db_path", filepath.Join(os.Getenv("HOME"), ".forge", "forge.db"))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Expand environment variables in API keys
	for name, p := range cfg.Providers {
		if strings.HasPrefix(p.APIKey, "${") && strings.HasSuffix(p.APIKey, "}") {
			envVar := p.APIKey[2 : len(p.APIKey)-1]
			p.APIKey = os.Getenv(envVar)
			cfg.Providers[name] = p
		}
	}

	return &cfg, nil
}

// IsOllama returns true if this provider looks like an Ollama instance.
func (p ProviderConfig) IsOllama() bool {
	return strings.Contains(p.BaseURL, ":11434") || strings.Contains(strings.ToLower(p.BaseURL), "ollama")
}

// Provider returns the config for a named provider, falling back to the default.
func (c *Config) Provider(name string) (ProviderConfig, error) {
	if name == "" {
		name = c.DefaultProvider
	}
	p, ok := c.Providers[name]
	if !ok {
		return ProviderConfig{}, fmt.Errorf("unknown provider: %s", name)
	}
	return p, nil
}
