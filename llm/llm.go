package llm

import "context"

type LLMConfig struct {
	BaseURL string
	Model   string
	APIKey  string
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ToolCall struct {
	Name string
	Args map[string]any
}

type Response struct {
	Content  string
	ToolCall *ToolCall
}

type LLM interface {
	Generate(ctx context.Context, messages []Message) (*Response, error)
}
