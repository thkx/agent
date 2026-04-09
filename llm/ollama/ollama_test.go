package ollama

import (
	"context"
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
