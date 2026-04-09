package agent

import (
	"context"
	"fmt"

	"github.com/thkx/agent/model"
	"github.com/thkx/agent/toolbus"
)

const (
	maxPlannerExec = 5
	maxToolExec    = 3
)

type Runtime struct {
	toolBus toolbus.Caller
	planner Planner
}

func NewRuntime(planner Planner, toolBus toolbus.Caller) *Runtime {
	return &Runtime{
		toolBus: toolBus,
		planner: planner,
	}
}

func (r *Runtime) Think(ctx context.Context, state *model.State) (*model.State, error) {
	if state.Counts == nil {
		state.Counts = make(map[string]int)
	}
	state.Counts["agent.llm"]++
	if state.Counts["agent.llm"] > maxPlannerExec {
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: "LLM node executed too many times, stopping.",
		})
		state.Action = model.ActionEnd
		state.PendingToolCall = nil
		return state, nil
	}

	if r.planner == nil {
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: "Planner not configured",
		})
		state.Action = model.ActionEnd
		state.PendingToolCall = nil
		return state, nil
	}

	plan, err := r.planner.Plan(ctx, state)
	if err != nil {
		state.Messages = append(state.Messages, plannerErrorMessage(err))
		state.Action = model.ActionEnd
		state.PendingToolCall = nil
		return state, nil
	}

	state.Messages = append(state.Messages, plan.Message)
	state.Action = plan.Action
	state.PendingToolCall = plan.ToolCall

	return state, nil
}

func (r *Runtime) CallTool(ctx context.Context, state *model.State) (*model.State, error) {
	if state.Counts == nil {
		state.Counts = make(map[string]int)
	}

	tc := state.PendingToolCall
	if tc == nil {
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
		return state, nil
	}

	state.Counts["agent.tool"]++
	if state.Counts["agent.tool"] > maxToolExec {
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: "Tool node executed too many times, stopping.",
		})
		state.Action = model.ActionEnd
		state.PendingToolCall = nil
		return state, nil
	}

	result, err := r.toolBus.Call(ctx, tc)
	if err != nil {
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: fmt.Sprintf("Tool %s failed: %v", tc.Name, err),
		})
		state.Action = model.ActionEnd
		state.PendingToolCall = nil
		return state, nil
	}

	state.Messages = append(state.Messages, model.Message{
		Role:    "tool",
		Content: fmt.Sprintf("%v", result),
	})

	state.Action = model.ActionLLM
	state.PendingToolCall = nil
	return state, nil
}
