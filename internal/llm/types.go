package llm

// Role represents a chat message role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single message in a conversation.
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // For tool result messages
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Args     map[string]any    `json:"arguments"`
}

// ToolDef defines a tool that the LLM can call.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
}

// StreamHandler receives text deltas during streaming.
// (Defined here for interface usage; implementation in stream.go)

// Response is the result of a chat completion call.
type Response struct {
	Message Message
}

// ModelInfo describes a model available on the provider.
type ModelInfo struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

// Helper constructors

func SystemMessage(content string) Message {
	return Message{Role: RoleSystem, Content: content}
}

func UserMessage(content string) Message {
	return Message{Role: RoleUser, Content: content}
}

func AssistantMessage(content string) Message {
	return Message{Role: RoleAssistant, Content: content}
}

func ToolResultMessage(toolCallID, content string) Message {
	return Message{Role: RoleTool, Content: content, ToolCallID: toolCallID}
}
