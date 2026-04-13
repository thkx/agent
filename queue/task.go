package queue

import (
	"context"
	"errors"

	"github.com/thkx/agent/model"
)

type TaskQueue struct {
	Queuer
}

func NewTaskQueue(size int) *TaskQueue {
	return &TaskQueue{
		Queuer: NewMemoryQueue(size),
	}
}

func (q *TaskQueue) PushTask(ctx context.Context, t *model.Task) error {
	return q.Push(ctx, t)
}

func (q *TaskQueue) PopTask(ctx context.Context) (*model.Task, error) {
	v, err := q.Pop(ctx)
	if err != nil {
		return nil, err
	}
	t, ok := v.(*model.Task)
	if !ok {
		return nil, errors.New("invalid task type")
	}
	return t, nil
}

// RetryTask returns true when the task has been scheduled for retry.
func (q *TaskQueue) RetryTask(t *model.Task) bool {
	if t.Retry < model.MAX_RETRY {
		retryTask := t.Clone()
		retryTask.Retry++
		q.Queuer.(*MemoryQueue).Retry(retryTask, retryTask.Retry)
		return true
	}
	return false
}
