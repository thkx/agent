package queue

import (
	"context"
	"errors"

	"github.com/thkx/agent/model"
)

type ResultQueue struct {
	q *MemoryQueue
}

func NewResultQueue(size int) *ResultQueue {
	return &ResultQueue{q: NewMemoryQueue(size)}
}

func (q *ResultQueue) PushResult(ctx context.Context, r *model.TaskResult) error {
	return q.q.Push(ctx, r)
}

func (q *ResultQueue) PopResult(ctx context.Context) (*model.TaskResult, error) {
	v, err := q.q.Pop(ctx)
	if err != nil {
		return nil, err
	}

	r, ok := v.(*model.TaskResult)
	if !ok {
		return nil, errors.New("invalid result type")
	}
	return r, nil
}
