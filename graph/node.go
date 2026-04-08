package graph

import (
	"context"

	"github.com/thkx/agent/model"
)

type Node interface {
	Name() string
	Execute(ctx context.Context, state *model.State) (*model.State, error)
}
