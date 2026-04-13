package agent

import (
	"context"

	"github.com/thkx/agent/model"
)

type ToolNode struct {
	runtime *Runtime
}

func (n *ToolNode) Name() string { return "tool" }

func (n *ToolNode) Execute(ctx context.Context, state *model.State) (*model.State, error) {
	return n.runtime.CallTool(ctx, state)
}
