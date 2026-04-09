package agent

import (
	"context"
	"testing"

	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/model"
)

type sequenceLLM struct {
	responses []*llm.Response
	index     int
}

func (s *sequenceLLM) Generate(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	resp := s.responses[s.index]
	s.index++
	return resp, nil
}

func TestAgentNodeRunsToolLoopInternally(t *testing.T) {
	t.Parallel()

	node := &AgentNode{
		runtime: NewRuntime(
			NewLLMPlanner(
				&sequenceLLM{
					responses: []*llm.Response{
						{Content: `{"tool":"get_price","args":{"symbol":"BTC"}}`},
						{Content: "BTC price is 65000"},
					},
				},
				nil,
				map[string]bool{"get_price": true},
			),
			&stubBus{},
		),
	}

	state := &model.State{
		Counts: map[string]int{},
		Messages: []model.Message{
			{Role: "user", Content: "price"},
		},
	}

	nextState, err := node.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if nextState.Action != model.ActionEnd {
		t.Fatalf("expected final action %q, got %q", model.ActionEnd, nextState.Action)
	}
	if len(nextState.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %#v", nextState.Messages)
	}
	if nextState.Messages[1].Role != "assistant" || nextState.Messages[2].Role != "tool" || nextState.Messages[3].Role != "assistant" {
		t.Fatalf("unexpected message sequence: %#v", nextState.Messages)
	}
}
