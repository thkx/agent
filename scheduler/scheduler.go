package scheduler

import (
	"context"

	"github.com/thkx/agent/model"
	"github.com/thkx/agent/queue"
)

type Scheduler struct {
	queue queue.Queuer
}

func New(queue queue.Queuer) *Scheduler {
	return &Scheduler{queue: queue}
}

func (s *Scheduler) Schedule(ctx context.Context, task *model.Task) error {
	return s.queue.Push(ctx, task)
}
