package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/tool"
)

func TestLLMPlannerBuildsToolDecision(t *testing.T) {
	t.Parallel()

	planner := NewLLMPlanner(
		&stubLLM{
			resp: &llm.Response{
				Content: `{"tool":"get_price","args":{"symbol":"BTC"}}`,
			},
		},
		nil,
		map[string]bool{"get_price": true},
	)

	result, err := planner.Plan(context.Background(), &model.State{
		Messages: []model.Message{{Role: "user", Content: "price"}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Action != model.ActionTool {
		t.Fatalf("expected action %q, got %q", model.ActionTool, result.Action)
	}
	if result.ToolCall == nil || result.ToolCall.Name != "get_price" {
		t.Fatalf("expected tool call to be set, got %#v", result.ToolCall)
	}
}

func TestLLMPlannerBuildsToolDecisionFromShorthandJSON(t *testing.T) {
	t.Parallel()

	planner := NewLLMPlanner(
		&stubLLM{
			resp: &llm.Response{
				Content: `{"get_price":"BTC"}`,
			},
		},
		tool.NewRegistry(&plannerTool{name: "get_price"}),
		map[string]bool{"get_price": true},
	)

	result, err := planner.Plan(context.Background(), &model.State{
		Messages: []model.Message{{Role: "user", Content: "price"}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Action != model.ActionTool {
		t.Fatalf("expected action %q, got %q", model.ActionTool, result.Action)
	}
	if result.ToolCall == nil || result.ToolCall.Name != "get_price" {
		t.Fatalf("expected tool call to be set, got %#v", result.ToolCall)
	}
	if result.ToolCall.Args["symbol"] != "BTC" {
		t.Fatalf("expected shorthand JSON to map into symbol arg, got %#v", result.ToolCall.Args)
	}
}

func TestLLMPlannerBuildsToolDecisionFromFencedJSON(t *testing.T) {
	t.Parallel()

	planner := NewLLMPlanner(
		&stubLLM{
			resp: &llm.Response{
				Content: "```json\n{\"tool\":\"get_price\"}\n```",
			},
		},
		tool.NewRegistry(&plannerTool{name: "get_price"}),
		map[string]bool{"get_price": true},
	)

	result, err := planner.Plan(context.Background(), &model.State{
		Messages: []model.Message{{Role: "user", Content: "price"}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Action != model.ActionTool {
		t.Fatalf("expected action %q, got %q", model.ActionTool, result.Action)
	}
	if result.ToolCall == nil || result.ToolCall.Name != "get_price" {
		t.Fatalf("expected tool call to be set, got %#v", result.ToolCall)
	}
}

func TestLLMPlannerInjectsToolDefinitionsIntoMessages(t *testing.T) {
	t.Parallel()

	llmClient := &capturingLLM{
		resp: &llm.Response{
			Content: "final answer",
		},
	}
	planner := NewLLMPlanner(
		llmClient,
		tool.NewRegistry(&plannerTool{name: "get_price"}),
		map[string]bool{"get_price": true},
	)

	_, err := planner.Plan(context.Background(), &model.State{
		Messages: []model.Message{{Role: "user", Content: "price"}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(llmClient.messages) == 0 {
		t.Fatal("expected planner to send messages to llm")
	}
	if llmClient.messages[0].Role != "system" {
		t.Fatalf("expected first message to be system, got %#v", llmClient.messages[0])
	}
	if !strings.Contains(llmClient.messages[0].Content, "get_price") || !strings.Contains(llmClient.messages[0].Content, "symbol") {
		t.Fatalf("expected tool definition prompt, got %q", llmClient.messages[0].Content)
	}
}

type capturingLLM struct {
	resp     *llm.Response
	messages []llm.Message
}

func (c *capturingLLM) Generate(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	c.messages = append([]llm.Message(nil), messages...)
	return c.resp, nil
}

type plannerTool struct {
	name string
}

func (t *plannerTool) Name() string { return t.name }

func (t *plannerTool) Description() string { return "Get a price" }

func (t *plannerTool) Schema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Property{
			"symbol": {Type: "string"},
		},
	}
}

func (t *plannerTool) Invoke(ctx context.Context, input any) (any, error) { return nil, nil }
