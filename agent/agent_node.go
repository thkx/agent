package agent

import (
	"context"

	"github.com/thkx/agent/model"
)

type AgentNode struct {
	runtime *Runtime
}

func (n *AgentNode) Name() string { return "agent" }

func (n *AgentNode) Execute(ctx context.Context, state *model.State) (*model.State, error) {
	for {
		var err error

		state, err = n.runtime.Think(ctx, state)
		if err != nil {
			return state, err
		}
		if state.Action != model.ActionTool {
			return state, nil
		}

		state, err = n.runtime.CallTool(ctx, state)
		if err != nil {
			return state, err
		}
		if state.Action != model.ActionLLM {
			return state, nil
		}
	}
}
