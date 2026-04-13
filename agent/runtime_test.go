package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/tracer"
)

type stubLLM struct {
	resp *llm.Response
	err  error
}

func (s *stubLLM) Generate(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	return s.resp, s.err
}

type stubBus struct {
	mu    sync.Mutex
	call  *llm.ToolCall
	calls []llm.ToolCall
	err   error // 新增：模拟错误
}

func (b *stubBus) Call(ctx context.Context, call *llm.ToolCall) (any, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.call = call
	if call != nil {
		b.calls = append(b.calls, *call)
	}
	if b.err != nil {
		return nil, b.err
	}
	return "ok", nil
}

func TestRuntimeCallToolUsesBus(t *testing.T) {
	t.Parallel()

	rt := NewRuntime(nil, &stubBus{}, nil)
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

func TestRuntimeCallToolUsesAllPendingToolCalls(t *testing.T) {
	t.Parallel()

	bus := &stubBus{}
	rt := NewRuntime(nil, bus, nil)
	state := &model.State{
		Counts: map[string]int{},
		PendingToolCalls: []llm.ToolCall{
			{ID: "call_price", Name: "price", Args: map[string]any{"symbol": "BTC"}},
			{ID: "call_volume", Name: "volume", Args: map[string]any{"symbol": "BTC"}},
		},
	}

	nextState, err := rt.CallTool(context.Background(), state)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if nextState.Action != model.ActionLLM {
		t.Fatalf("expected next action %q, got %q", model.ActionLLM, nextState.Action)
	}
	if len(bus.calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %#v", bus.calls)
	}
	if len(nextState.Messages) != 2 || nextState.Messages[0].Role != "tool" || nextState.Messages[1].Role != "tool" {
		t.Fatalf("expected 2 tool messages, got %#v", nextState.Messages)
	}
	if nextState.Messages[0].ToolCallID == "" || nextState.Messages[1].ToolCallID == "" {
		t.Fatalf("expected tool call ids on tool messages, got %#v", nextState.Messages)
	}
	if nextState.PendingToolCalls != nil {
		t.Fatal("expected pending tool calls to be cleared")
	}
}

func TestRuntimeEmitsTracingSpans(t *testing.T) {
	t.Parallel()

	memTracer := tracer.NewMemory()
	rt := NewRuntime(NewLLMPlanner(&stubLLM{
		resp: &llm.Response{Content: "done"},
	}, nil, nil), &stubBus{}, memTracer)

	state := &model.State{
		Counts: map[string]int{},
		Messages: []model.Message{
			{Role: "user", Content: "hi"},
		},
	}

	if _, err := rt.Think(context.Background(), state); err != nil {
		t.Fatalf("think failed: %v", err)
	}

	state.PendingToolCall = &llm.ToolCall{Name: "test_tool", Args: map[string]any{"symbol": "BTC"}}
	state.PendingToolCalls = []llm.ToolCall{{Name: "test_tool", Args: map[string]any{"symbol": "BTC"}}}
	if _, err := rt.CallTool(context.Background(), state); err != nil {
		t.Fatalf("call tool failed: %v", err)
	}

	spans := memTracer.Spans()
	var sawThink bool
	var sawCallTool bool
	for _, span := range spans {
		if span.Name == "agent.think" {
			sawThink = true
		}
		if span.Name == "agent.call_tool" {
			sawCallTool = true
		}
	}
	if !sawThink || !sawCallTool {
		t.Fatalf("expected think and call_tool spans, got %#v", spans)
	}
}

func TestRuntimeConfigurableLimits(t *testing.T) {
	t.Parallel()

	// 测试自定义最大规划器执行次数
	rt := NewRuntime(nil, &stubBus{}, nil, WithMaxPlannerExec(2))
	state := &model.State{
		Counts: map[string]int{"agent.llm": 2}, // 已经达到限制
		Messages: []model.Message{
			{Role: "user", Content: "hi"},
		},
	}

	nextState, err := rt.Think(context.Background(), state)
	if err != nil {
		t.Fatalf("think failed: %v", err)
	}
	if nextState.Action != model.ActionEnd {
		t.Fatalf("expected action end due to limit, got %q", nextState.Action)
	}
	if len(nextState.Messages) < 2 || nextState.Messages[len(nextState.Messages)-1].Content != "LLM node executed too many times, stopping." {
		t.Fatalf("expected stop message, got messages: %#v", nextState.Messages)
	}
}

func TestRuntimeConfigurableToolLimits(t *testing.T) {
	t.Parallel()

	// 测试自定义最大工具执行次数
	rt := NewRuntime(nil, &stubBus{}, nil, WithMaxToolExec(1))
	state := &model.State{
		Counts: map[string]int{"agent.tool": 1}, // 已经达到限制
		PendingToolCalls: []llm.ToolCall{
			{Name: "test_tool", Args: map[string]any{"symbol": "BTC"}},
		},
	}

	nextState, err := rt.CallTool(context.Background(), state)
	if err != nil {
		t.Fatalf("call tool failed: %v", err)
	}
	if nextState.Action != model.ActionEnd {
		t.Fatalf("expected action end due to limit, got %q", nextState.Action)
	}
	if len(nextState.Messages) < 1 || nextState.Messages[len(nextState.Messages)-1].Content != "Tool node executed too many times, stopping." {
		t.Fatalf("expected stop message, got messages: %#v", nextState.Messages)
	}
}

func TestRuntimeToolErrorHandling(t *testing.T) {
	t.Parallel()

	// 创建一个总是失败的bus
	failingBus := &stubBus{err: fmt.Errorf("simulated tool failure")}

	rt := NewRuntime(nil, failingBus, nil)
	state := &model.State{
		Counts: map[string]int{},
		PendingToolCalls: []llm.ToolCall{
			{Name: "failing_tool", Args: map[string]any{"arg": "value"}},
		},
	}

	nextState, err := rt.CallTool(context.Background(), state)
	if err != nil {
		t.Fatalf("call tool failed: %v", err)
	}
	if nextState.Action != model.ActionLLM {
		t.Fatalf("expected action to remain LLM, got %q", nextState.Action)
	}
	if len(nextState.Messages) < 2 {
		t.Fatalf("expected at least 2 messages (tool result and warning), got %d", len(nextState.Messages))
	}
	// 检查工具消息包含错误
	toolMsg := nextState.Messages[0]
	if toolMsg.Error == nil {
		t.Fatalf("expected tool message to have error, got nil")
	}
	if toolMsg.Content != "Tool failing_tool failed: simulated tool failure" {
		t.Fatalf("expected error content, got %q", toolMsg.Content)
	}
	// 检查警告消息
	warningMsg := nextState.Messages[1]
	if warningMsg.Content != "Some tools failed, but continuing execution." {
		t.Fatalf("expected warning message, got %q", warningMsg.Content)
	}
}
