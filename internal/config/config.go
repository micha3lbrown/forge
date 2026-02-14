package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type ProviderConfig struct {
	BaseURL string            `mapstructure:"base_url"`
	APIKey  string            `mapstructure:"api_key"`
	Models  map[string]string `mapstructure:"models"`
}

type AgentConfig struct {
	MaxIterations int `mapstructure:"max_iterations"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type Config struct {
	Providers       map[string]ProviderConfig `mapstructure:"providers"`
	DefaultProvider string                    `mapstructure:"default_provider"`
	Agent           AgentConfig               `mapstructure:"agent"`
	Server          ServerConfig              `mapstructure:"server"`
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
