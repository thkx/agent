package llm

import "context"

type LLMConfig struct {
	BaseURL string
	Model   string
	APIKey  string
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolName   string     `json:"tool_name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name"`
	Args map[string]any `json:"arguments,omitempty"`
}

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type Response struct {
	Content   string
	ToolCall  *ToolCall
	ToolCalls []ToolCall
}

type LLM interface {
	Generate(ctx context.Context, messages []Message) (*Response, error)
}

type StructuredToolCaller interface {
	GenerateWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (*Response, error)
}
