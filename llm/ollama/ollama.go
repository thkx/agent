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
	Model    string        `json:"model"`
	Messages []llm.Message `json:"messages"`
	Stream   bool          `json:"stream"`
}

type ollamaResp struct {
	Message llm.Message `json:"message"`
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
	reqBody := ollamaReq{
		Model:    o.Config.Model,
		Messages: messages,
		Stream:   false,
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
		Content: r.Message.Content,
	}, nil
}
