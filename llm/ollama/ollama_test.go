package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thkx/agent/llm"
)

func TestGenerateReturnsResponseContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"hello"}}`))
	}))
	defer server.Close()

	client := New(
		llm.WithModel("test-model"),
		llm.WithBaseURL(server.URL),
	)

	resp, err := client.Generate(context.Background(), []llm.Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resp.Content != "hello" {
		t.Fatalf("expected content %q, got %q", "hello", resp.Content)
	}
}

func TestGenerateReturnsHTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client := New(
		llm.WithModel("test-model"),
		llm.WithBaseURL(server.URL),
	)

	_, err := client.Generate(context.Background(), []llm.Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected http error")
	}
	if !strings.Contains(err.Error(), "status=400") {
		t.Fatalf("expected status code in error, got %v", err)
	}
}

func TestGenerateWithToolsParsesStructuredToolCall(t *testing.T) {
	t.Parallel()

	var body ollamaReq
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_btc","function":{"name":"get_price","arguments":{"symbol":"BTC"}}}]}}`))
	}))
	defer server.Close()

	client := New(
		llm.WithModel("test-model"),
		llm.WithBaseURL(server.URL),
	)

	resp, err := client.GenerateWithTools(context.Background(), []llm.Message{{Role: "user", Content: "hi"}}, []llm.ToolDefinition{
		{
			Name:        "get_price",
			Description: "Get a price",
			Parameters: map[string]any{
				"type": "object",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(body.Tools) != 1 || body.Tools[0].Function.Name != "get_price" {
		t.Fatalf("expected request to include tool definition, got %#v", body.Tools)
	}
	if resp.ToolCall == nil || resp.ToolCall.ID != "call_btc" || resp.ToolCall.Name != "get_price" || resp.ToolCall.Args["symbol"] != "BTC" {
		t.Fatalf("expected structured tool call response, got %#v", resp.ToolCall)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "get_price" {
		t.Fatalf("expected structured tool calls slice, got %#v", resp.ToolCalls)
	}
}

func TestGenerateWithToolsParsesMultipleStructuredToolCalls(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_btc","function":{"name":"get_price","arguments":{"symbol":"BTC"}}},{"id":"call_eth","function":{"name":"get_price","arguments":{"symbol":"ETH"}}}]}}`))
	}))
	defer server.Close()

	client := New(
		llm.WithModel("test-model"),
		llm.WithBaseURL(server.URL),
	)

	resp, err := client.GenerateWithTools(context.Background(), []llm.Message{{Role: "user", Content: "hi"}}, []llm.ToolDefinition{
		{Name: "get_price", Parameters: map[string]any{"type": "object"}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("expected 2 structured tool calls, got %#v", resp.ToolCalls)
	}
	if resp.ToolCalls[0].ID != "call_btc" || resp.ToolCalls[1].ID != "call_eth" {
		t.Fatalf("expected tool call ids to be preserved, got %#v", resp.ToolCalls)
	}
}
