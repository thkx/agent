package agent

import (
	"context"
	"testing"

	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/model"
)

type stubLLM struct {
	resp *llm.Response
	err  error
}

func (s *stubLLM) Generate(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return s.resp, s.err
}

type stubBus struct {
	call *llm.ToolCall
}

func (b *stubBus) Call(ctx context.Context, call *llm.ToolCall) (any, error) {
	b.call = call
	return "ok", nil
}

func TestRuntimeCallToolUsesBus(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(nil, &stubBus{})
	state := &model.State{
		Counts: map[string]int{},
		PendingToolCall: &llm.ToolCall{
			Name: "test_tool",
			Args: map[string]any{"symbol": "BTC"},
		},
	}

	nextState, err := rt.CallTool(context.Background(), state)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if nextState.Action != model.ActionLLM {
		t.Fatalf("expected next action %q, got %q", model.ActionLLM, nextState.Action)
	}
	if nextState.PendingToolCall != nil {
		t.Fatal("expected pending tool call to be cleared")
	}
}
