package agent

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Profile defines an agent's personality and capabilities.
type Profile struct {
	Name         string   `yaml:"name"`
	Provider     string   `yaml:"provider"`
	Model        string   `yaml:"model"`
	SystemPrompt string   `yaml:"system_prompt"`
	Tools        []string `yaml:"tools"`
	MaxIter      int      `yaml:"max_iterations"`
}

// LoadProfile reads an agent profile from a YAML file.
func LoadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading profile %s: %w", path, err)
	}

	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing profile %s: %w", path, err)
	}

	return &p, nil
}
