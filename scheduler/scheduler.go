package scheduler

import (
	"context"

	"github.com/thkx/agent/model"
	"github.com/thkx/agent/queue"
)

type Scheduler struct {
	taskQueue *queue.TaskQueue
}

func New(taskQueue *queue.TaskQueue) *Scheduler {
	return &Scheduler{taskQueue: taskQueue}
}

func (s *Scheduler) Submit(ctx context.Context, task *model.Task) error {
	return s.taskQueue.PushTask(ctx, task)
}
