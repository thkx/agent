package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/toolbus"
	"github.com/thkx/agent/tracer"
)

const (
	defaultMaxPlannerExec = 5
	defaultMaxToolExec    = 3
)

type Runtime struct {
	toolBus        toolbus.Caller
	planner        Planner
	tracer         tracer.Tracer
	maxPlannerExec int
	maxToolExec    int
}

type RuntimeOption func(*Runtime)

func WithMaxPlannerExec(max int) RuntimeOption {
	return func(r *Runtime) {
		r.maxPlannerExec = max
	}
}

func WithMaxToolExec(max int) RuntimeOption {
	return func(r *Runtime) {
		r.maxToolExec = max
	}
}

func NewRuntime(planner Planner, toolBus toolbus.Caller, t tracer.Tracer, opts ...RuntimeOption) *Runtime {
	if t == nil {
		t = tracer.NewNoop()
	}
	r := &Runtime{
		toolBus:        toolBus,
		planner:        planner,
		tracer:         t,
		maxPlannerExec: defaultMaxPlannerExec,
		maxToolExec:    defaultMaxToolExec,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Runtime) Think(ctx context.Context, state *model.State) (*model.State, error) {
	ctx, finish := r.tracer.StartSpan(ctx, tracer.Span{
		Name:  "agent.think",
		Input: state.Clone(),
	})
	defer finish(state.Clone(), nil)

	if state.Counts == nil {
		state.Counts = make(map[string]int)
	}
	state.Counts["agent.llm"]++
	if state.Counts["agent.llm"] > r.maxPlannerExec {
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: "LLM node executed too many times, stopping.",
		})
		state.Action = model.ActionEnd
		state.PendingToolCall = nil
		state.PendingToolCalls = nil
		return state, nil
	}

	if r.planner == nil {
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: "Planner not configured",
		})
		state.Action = model.ActionEnd
		state.PendingToolCall = nil
		state.PendingToolCalls = nil
		return state, nil
	}

	plan, err := r.planner.Plan(ctx, state)
	if err != nil {
		state.Messages = append(state.Messages, plannerErrorMessage(err))
		state.Action = model.ActionEnd
		state.PendingToolCall = nil
		state.PendingToolCalls = nil
		return state, nil
	}

	state.Messages = append(state.Messages, plan.Message)
	state.Action = plan.Action
	state.PendingToolCall = plan.ToolCall
	state.PendingToolCalls = cloneRuntimeToolCalls(plan.ToolCalls)
	if state.PendingToolCall == nil && len(state.PendingToolCalls) > 0 {
		state.PendingToolCall = &state.PendingToolCalls[0]
	}

	// 修剪消息以防止内存无限增长
	state.TrimMessages()

	return state, nil
}

func (r *Runtime) CallTool(ctx context.Context, state *model.State) (*model.State, error) {
	ctx, finish := r.tracer.StartSpan(ctx, tracer.Span{
		Name:  "agent.call_tool",
		Input: state.Clone(),
	})
	defer finish(state.Clone(), nil)

	if state.Counts == nil {
		state.Counts = make(map[string]int)
	}

	pending := cloneRuntimeToolCalls(state.PendingToolCalls)
	if len(pending) == 0 && state.PendingToolCall != nil {
		pending = []llm.ToolCall{*cloneToolCall(state.PendingToolCall)}
	}
	if len(pending) == 0 {
		state.Action = model.ActionEnd
		return state, nil
	}

	if r.toolBus == nil {
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: "Tool bus not configured",
		})
		state.Action = model.ActionEnd
		state.PendingToolCall = nil
		state.PendingToolCalls = nil
		return state, nil
	}

	state.Counts["agent.tool"] += len(pending)
	if state.Counts["agent.tool"] > r.maxToolExec {
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: "Tool node executed too many times, stopping.",
		})
		state.Action = model.ActionEnd
		state.PendingToolCall = nil
		state.PendingToolCalls = nil
		return state, nil
	}

	results := make([]model.Message, len(pending))
	var wg sync.WaitGroup

	for i, tc := range pending {
		wg.Add(1)
		go func(i int, tc llm.ToolCall) {
			defer wg.Done()
			// 为每个工具调用创建带超时的上下文
			toolCtx, cancel := context.WithTimeout(ctx, time.Minute) // 默认超时，可根据工具调整
			defer cancel()
			result, err := r.toolBus.Call(toolCtx, &tc)
			if err != nil {
				results[i] = model.Message{
					Role:       "tool",
					Content:    fmt.Sprintf("Tool %s failed: %v", tc.Name, err),
					ToolName:   tc.Name,
					ToolCallID: tc.ID,
					Error:      fmt.Errorf("Tool %s failed: %w", tc.Name, err), // 保留错误上下文
				}
				return
			}
			results[i] = model.Message{
				Role:       "tool",
				Content:    fmt.Sprintf("%v", result),
				ToolName:   tc.Name,
				ToolCallID: tc.ID,
				Error:      nil, // 成功时无错误
			}
		}(i, tc)
	}

	wg.Wait()

	// 检查是否有任何错误，如果有，决定是否继续
	hasErrors := false
	for _, msg := range results {
		if msg.Error != nil {
			hasErrors = true
			break
		}
	}

	// 如果有错误，可以选择停止或继续。这里选择继续，但记录错误
	state.Messages = append(state.Messages, results...)

	if hasErrors {
		// 添加警告消息，但不停止执行
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: "Some tools failed, but continuing execution.",
		})
	}

	state.Action = model.ActionLLM
	state.PendingToolCall = nil
	state.PendingToolCalls = nil

	// 修剪消息以防止内存无限增长
	state.TrimMessages()

	return state, nil
}

func cloneRuntimeToolCalls(calls []llm.ToolCall) []llm.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]llm.ToolCall, len(calls))
	for i, call := range calls {
		out[i] = *cloneToolCall(&call)
	}
	return out
}
