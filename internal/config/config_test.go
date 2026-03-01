package config

import (
	"testing"
)

func TestFallbackProviders_BasicChain(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"ollama": {BaseURL: "http://localhost:11434/v1/", APIKey: "ollama"},
			"claude": {BaseURL: "https://api.anthropic.com/v1/", APIKey: "sk-test", Models: map[string]string{"default": "claude-sonnet-4-5-20250929"}},
			"gemini": {BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai/", APIKey: "gm-test", Models: map[string]string{"default": "gemini-2.5-flash"}},
		},
		Fallback: map[string][]string{
			"ollama": {"gemini", "claude"},
			"gemini": {"claude"},
			"claude": {"gemini"},
		},
	}

	opts := cfg.FallbackProviders("ollama")
	if len(opts) != 2 {
		t.Fatalf("expected 2 fallback options, got %d", len(opts))
	}
	if opts[0].Provider != "gemini" || opts[0].Model != "gemini-2.5-flash" {
		t.Errorf("first fallback = %+v, want gemini/gemini-2.5-flash", opts[0])
	}
	if opts[1].Provider != "claude" || opts[1].Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("second fallback = %+v, want claude/claude-sonnet-4-5-20250929", opts[1])
	}
}

func TestFallbackProviders_FiltersEmptyAPIKeys(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"ollama": {BaseURL: "http://localhost:11434/v1/", APIKey: "ollama"},
			"claude": {BaseURL: "https://api.anthropic.com/v1/", APIKey: "", Models: map[string]string{"default": "claude-sonnet-4-5-20250929"}},
			"gemini": {BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai/", APIKey: "gm-test", Models: map[string]string{"default": "gemini-2.5-flash"}},
		},
		Fallback: map[string][]string{
			"ollama": {"claude", "gemini"},
		},
	}

	opts := cfg.FallbackProviders("ollama")
	if len(opts) != 1 {
		t.Fatalf("expected 1 fallback option (claude filtered), got %d", len(opts))
	}
	if opts[0].Provider != "gemini" {
		t.Errorf("expected gemini, got %s", opts[0].Provider)
	}
}

func TestFallbackProviders_OllamaSkipsKeyCheck(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"claude": {BaseURL: "https://api.anthropic.com/v1/", APIKey: "sk-test", Models: map[string]string{"default": "claude-sonnet-4-5-20250929"}},
			"ollama": {BaseURL: "http://localhost:11434/v1/", APIKey: "", Models: map[string]string{"default": "qwen3:14b"}},
		},
		Fallback: map[string][]string{
			"claude": {"ollama"},
		},
	}

	opts := cfg.FallbackProviders("claude")
	if len(opts) != 1 {
		t.Fatalf("expected 1 fallback (ollama needs no key), got %d", len(opts))
	}
	if opts[0].Provider != "ollama" {
		t.Errorf("expected ollama, got %s", opts[0].Provider)
	}
}

func TestFallbackProviders_EmptyConfig(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"ollama": {BaseURL: "http://localhost:11434/v1/"},
		},
	}

	opts := cfg.FallbackProviders("ollama")
	if opts != nil {
		t.Errorf("expected nil for empty fallback config, got %v", opts)
	}
}

func TestFallbackProviders_UnknownProvider(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"ollama": {BaseURL: "http://localhost:11434/v1/"},
		},
		Fallback: map[string][]string{
			"ollama": {"nonexistent"},
		},
	}

	opts := cfg.FallbackProviders("ollama")
	if len(opts) != 0 {
		t.Errorf("expected 0 options for unknown fallback provider, got %d", len(opts))
	}
}
