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
	return n.runtime.Think(ctx, state)
}
