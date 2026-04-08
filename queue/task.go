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

func (q *TaskQueue) PushTask(t *model.Task) error {
	return q.Push(context.Background(), t)
}

func (q *TaskQueue) PopTask() (*model.Task, error) {
	v, err := q.Pop(context.Background())
	if err != nil {
		return nil, err
	}
	t, ok := v.(*model.Task)
	if !ok {
		return nil, errors.New("invalid task type")
	}
	return t, nil
}

// Retry task
func (q *TaskQueue) RetryTask(t *model.Task) {
	if t.Retry < model.MAX_RETRY {
		t.Retry++
		q.Queuer.(*MemoryQueue).Retry(t, t.Retry)
	}
}
