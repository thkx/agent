package agent

import (
	"context"
	"testing"
	"time"

	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/runtime"
	"github.com/thkx/agent/scheduler"
	"github.com/thkx/agent/tool"
	"github.com/thkx/agent/worker"
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

type graphLoopTool struct{}

func (t *graphLoopTool) Name() string { return "get_price" }

func (t *graphLoopTool) Description() string { return "Get a price" }

func (t *graphLoopTool) Schema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Property{
			"symbol": {Type: "string"},
		},
		Required: []string{"symbol"},
	}
}

func (t *graphLoopTool) Invoke(ctx context.Context, input any) (any, error) {
	args, _ := input.(map[string]any)
	symbol, _ := args["symbol"].(string)
	if symbol == "" {
		symbol = "BTC"
	}
	return symbol + " price is 65000", nil
}

func (t *graphLoopTool) Timeout() time.Duration {
	return 10 * time.Second
}

func (t *graphLoopTool) Permissions() []string {
	return []string{"read"}
}

func TestBuildGraphRoutesAgentLoopAcrossNodes(t *testing.T) {
	t.Parallel()

	ag := New(nil)
	g := ag.buildGraph()

	next, err := g.Next("agent", &model.State{Action: model.ActionTool})
	if err != nil || next != "tool" {
		t.Fatalf("expected agent -> tool, got next=%q err=%v", next, err)
	}

	next, err = g.Next("tool", &model.State{Action: model.ActionLLM})
	if err != nil || next != "agent" {
		t.Fatalf("expected tool -> agent, got next=%q err=%v", next, err)
	}

	next, err = g.Next("agent", &model.State{Action: model.ActionEnd})
	if err != nil || next != "end" {
		t.Fatalf("expected agent -> end, got next=%q err=%v", next, err)
	}
}

func TestAgentRunLoopsAcrossGraphNodes(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	taskQ := queue.NewTaskQueue(8)
	resultQ := queue.NewResultQueue(8)
	graphStore := runtime.NewMemoryGraphStore()
	rt := runtime.New(
		runtime.WithScheduler(scheduler.New(taskQ)),
		runtime.WithResultQueue(resultQ),
		runtime.WithGraphStore(graphStore),
	)

	w := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphStore(graphStore),
		worker.WithConcurrency(1),
	)
	go w.Start(ctx)

	ag := New(rt).
		WithLLM(&sequenceLLM{
			responses: []*llm.Response{
				{ToolCall: &llm.ToolCall{Name: "get_price", Args: map[string]any{"symbol": "BTC"}}},
				{Content: "BTC price is 65000"},
			},
		}).
		WithTools(&graphLoopTool{})

	result, err := ag.Run(ctx, "price")
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if result != "BTC price is 65000" {
		t.Fatalf("unexpected final result: %q", result)
	}
}

func TestAgentRunHandlesMultipleToolCallsInOneStep(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	taskQ := queue.NewTaskQueue(8)
	resultQ := queue.NewResultQueue(8)
	graphStore := runtime.NewMemoryGraphStore()
	rt := runtime.New(
		runtime.WithScheduler(scheduler.New(taskQ)),
		runtime.WithResultQueue(resultQ),
		runtime.WithGraphStore(graphStore),
	)

	w := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphStore(graphStore),
		worker.WithConcurrency(2),
	)
	go w.Start(ctx)

	ag := New(rt).
		WithLLM(&sequenceLLM{
			responses: []*llm.Response{
				{ToolCalls: []llm.ToolCall{
					{Name: "get_price", Args: map[string]any{"symbol": "BTC"}},
					{Name: "get_price", Args: map[string]any{"symbol": "ETH"}},
				}},
				{Content: "BTC and ETH done"},
			},
		}).
		WithTools(&graphLoopTool{})

	result, err := ag.Run(ctx, "prices")
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if result != "BTC and ETH done" {
		t.Fatalf("unexpected final result: %q", result)
	}
}
