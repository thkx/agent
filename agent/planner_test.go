package agent

import (
	"context"
	"strings"
	"testing"
	"time"

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

func TestLLMPlannerBuildsMultipleToolDecisionsFromJSON(t *testing.T) {
	t.Parallel()

	planner := NewLLMPlanner(
		&stubLLM{
			resp: &llm.Response{
				Content: `{"tool_calls":[{"tool":"get_price","args":{"symbol":"BTC"}},{"tool":"get_price","args":{"symbol":"ETH"}}]}`,
			},
		},
		tool.NewRegistry(&plannerTool{name: "get_price"}),
		map[string]bool{"get_price": true},
	)

	result, err := planner.Plan(context.Background(), &model.State{
		Messages: []model.Message{{Role: "user", Content: "prices"}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Action != model.ActionTool || len(result.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %#v", result)
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

func TestLLMPlannerUsesStructuredToolCallingWhenAvailable(t *testing.T) {
	t.Parallel()

	llmClient := &structuredCapturingLLM{
		resp: &llm.Response{
			ToolCall: &llm.ToolCall{
				Name: "get_price",
				Args: map[string]any{"symbol": "BTC"},
			},
		},
	}
	planner := NewLLMPlanner(
		llmClient,
		tool.NewRegistry(&plannerTool{name: "get_price"}),
		map[string]bool{"get_price": true},
	)

	result, err := planner.Plan(context.Background(), &model.State{
		Messages: []model.Message{{Role: "user", Content: "price"}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Action != model.ActionTool || result.ToolCall == nil || result.ToolCall.Name != "get_price" {
		t.Fatalf("expected structured tool decision, got %#v", result)
	}
	if len(llmClient.tools) != 1 || llmClient.tools[0].Name != "get_price" {
		t.Fatalf("expected structured llm to receive tools, got %#v", llmClient.tools)
	}
	if len(llmClient.messages) == 0 || strings.Contains(llmClient.messages[0].Content, "Available tools:") {
		t.Fatalf("expected structured path to avoid catalog prompt, got %#v", llmClient.messages)
	}
}

func TestLLMPlannerUsesMultipleStructuredToolCallsWhenAvailable(t *testing.T) {
	t.Parallel()

	llmClient := &structuredCapturingLLM{
		resp: &llm.Response{
			ToolCalls: []llm.ToolCall{
				{Name: "get_price", Args: map[string]any{"symbol": "BTC"}},
				{Name: "get_price", Args: map[string]any{"symbol": "ETH"}},
			},
		},
	}
	planner := NewLLMPlanner(
		llmClient,
		tool.NewRegistry(&plannerTool{name: "get_price"}),
		map[string]bool{"get_price": true},
	)

	result, err := planner.Plan(context.Background(), &model.State{
		Messages: []model.Message{{Role: "user", Content: "prices"}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Action != model.ActionTool || len(result.ToolCalls) != 2 {
		t.Fatalf("expected 2 structured tool decisions, got %#v", result)
	}
	if result.ToolCalls[0].ID == "" || result.ToolCalls[1].ID == "" {
		t.Fatalf("expected planner to assign tool call ids, got %#v", result.ToolCalls)
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

type structuredCapturingLLM struct {
	resp     *llm.Response
	messages []llm.Message
	tools    []llm.ToolDefinition
}

func (c *structuredCapturingLLM) Generate(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	c.messages = append([]llm.Message(nil), messages...)
	return c.resp, nil
}

func (c *structuredCapturingLLM) GenerateWithTools(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition) (*llm.Response, error) {
	c.messages = append([]llm.Message(nil), messages...)
	c.tools = append([]llm.ToolDefinition(nil), tools...)
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

func (t *plannerTool) Timeout() time.Duration {
	return 10 * time.Second
}

func (t *plannerTool) Permissions() []string {
	return []string{"read"}
}
