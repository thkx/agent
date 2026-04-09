package toolbus

import (
	"context"
	"testing"

	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/toolruntime"
)

type stubRuntime struct {
	call toolruntime.ToolCall
}

func (r *stubRuntime) Execute(ctx context.Context, call toolruntime.ToolCall) (toolruntime.ToolResult, error) {
	r.call = call
	return toolruntime.ToolResult{Output: "ok"}, nil
}

func TestToolBusCall(t *testing.T) {
	t.Parallel()

	rt := &stubRuntime{}
	bus := New(rt)

	result, err := bus.Call(context.Background(), &llm.ToolCall{
		Name: "tool_a",
		Args: map[string]any{"x": 1},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected result %q, got %#v", "ok", result)
	}
	if rt.call.Name != "tool_a" {
		t.Fatalf("expected routed call name %q, got %q", "tool_a", rt.call.Name)
	}
	input, ok := rt.call.Input.(map[string]any)
	if !ok || input["x"] != 1 {
		t.Fatalf("expected routed input payload, got %#v", rt.call.Input)
	}
}
