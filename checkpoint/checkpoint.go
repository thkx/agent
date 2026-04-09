package checkpoint

import (
	"context"

	"github.com/thkx/agent/model"
)

type Checkpointer interface {
	Save(context.Context, *model.ExecutionSnapshot)
	Load(context.Context, string) *model.ExecutionSnapshot
}

type noopCheckpoint struct{}

func NewNoopCheckpoint() Checkpointer {
	return &noopCheckpoint{}
}

func (n *noopCheckpoint) Save(ctx context.Context, snapshot *model.ExecutionSnapshot) {
	// do nothing
}

func (n *noopCheckpoint) Load(ctx context.Context, execID string) *model.ExecutionSnapshot {
	return nil
}
