package checkpoint

import (
	"context"

	"github.com/thkx/agent/model"
)

type Checkpointer interface {
	Save(context.Context, string, string, *model.State)
	Load(context.Context, string) *model.Task
}

type noopCheckpoint struct{}

func NewNoopCheckpoint() Checkpointer {
	return &noopCheckpoint{}
}

func (n *noopCheckpoint) Save(ctx context.Context, execID, node string, state *model.State) {
	// do nothing
}

func (n *noopCheckpoint) Load(ctx context.Context, execID string) *model.Task {
	return nil
}
