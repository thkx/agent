package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thkx/agent/llm"
)

const base_url = "http://localhost:11434"

type ollama struct {
	Config *llm.LLMConfig
}

type ollamaReq struct {
	Model    string             `json:"model"`
	Messages []ollamaReqMessage `json:"messages"`
	Stream   bool               `json:"stream"`
	Tools    []toolDef          `json:"tools,omitempty"`
}

type ollamaResp struct {
	Message ollamaMessage `json:"message"`
}

type ollamaMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolName   string           `json:"tool_name,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaReqMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolName   string           `json:"tool_name,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	ID       string             `json:"id,omitempty"`
	Function ollamaFunctionCall `json:"function"`
}

type ollamaFunctionCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type toolDef struct {
	Type     string      `json:"type"`
	Function functionDef `json:"function"`
}

type functionDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

func New(opts ...llm.Option) *ollama {
	ollm := &ollama{
		Config: &llm.LLMConfig{},
	}
	for _, opt := range opts {
		opt(ollm.Config)
	}

	if ollm.Config.BaseURL == "" {
		ollm.Config.BaseURL = base_url
	}

	if ollm.Config.Model == "" {
		panic("Model required")
	}

	return ollm
}

func (o *ollama) Generate(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return o.generate(ctx, messages, nil)
}

func (o *ollama) GenerateWithTools(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition) (*llm.Response, error) {
	return o.generate(ctx, messages, tools)
}

func (o *ollama) generate(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition) (*llm.Response, error) {
	reqBody := ollamaReq{
		Model:    o.Config.Model,
		Messages: toOllamaMessages(messages),
		Stream:   false,
	}
	if len(tools) > 0 {
		reqBody.Tools = make([]toolDef, 0, len(tools))
		for _, tool := range tools {
			reqBody.Tools = append(reqBody.Tools, toolDef{
				Type: "function",
				Function: functionDef{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.Parameters,
				},
			})
		}
	}

	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.Config.BaseURL+"/api/chat", bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama request failed: status=%d body=%s", resp.StatusCode, bytes.TrimSpace(body))
	}

	var r ollamaResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode ollama response: %w", err)
	}

	return &llm.Response{
		Content:   r.Message.Content,
		ToolCall:  parseToolCall(r.Message.ToolCalls),
		ToolCalls: parseToolCalls(r.Message.ToolCalls),
	}, nil
}

func toOllamaMessages(messages []llm.Message) []ollamaReqMessage {
	out := make([]ollamaReqMessage, 0, len(messages))
	for _, msg := range messages {
		item := ollamaReqMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolName:   msg.ToolName,
			ToolCallID: msg.ToolCallID,
		}
		if len(msg.ToolCalls) > 0 {
			item.ToolCalls = make([]ollamaToolCall, 0, len(msg.ToolCalls))
			for _, call := range msg.ToolCalls {
				item.ToolCalls = append(item.ToolCalls, ollamaToolCall{
					ID: call.ID,
					Function: ollamaFunctionCall{
						Name:      call.Name,
						Arguments: call.Args,
					},
				})
			}
		}
		out = append(out, item)
	}
	return out
}

func parseToolCall(calls []ollamaToolCall) *llm.ToolCall {
	if len(calls) == 0 {
		return nil
	}

	call := calls[0].Function
	if call.Name == "" {
		return nil
	}
	if call.Arguments == nil {
		call.Arguments = map[string]any{}
	}
	return &llm.ToolCall{
		ID:   calls[0].ID,
		Name: call.Name,
		Args: call.Arguments,
	}
}

func parseToolCalls(calls []ollamaToolCall) []llm.ToolCall {
	if len(calls) == 0 {
		return nil
	}

	out := make([]llm.ToolCall, 0, len(calls))
	for _, item := range calls {
		call := item.Function
		if call.Name == "" {
			continue
		}
		if call.Arguments == nil {
			call.Arguments = map[string]any{}
		}
		out = append(out, llm.ToolCall{
			ID:   item.ID,
			Name: call.Name,
			Args: call.Arguments,
		})
	}
	return out
}
