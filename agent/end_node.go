package agent

import (
	"context"

	"github.com/thkx/agent/model"
)

type EndNode struct{}

func (n *EndNode) Name() string {
	return "end"
}

func (n *EndNode) Execute(ctx context.Context, state *model.State) (*model.State, error) {
	// 什么都不做，直接结束
	return state, nil
}
