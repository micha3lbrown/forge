package tools

// ToolServerConfig describes an MCP tool server binary.
type ToolServerConfig struct {
	Binary  string            `mapstructure:"binary"`
	Env     map[string]string `mapstructure:"env"`
	Enabled bool              `mapstructure:"enabled"`
}
