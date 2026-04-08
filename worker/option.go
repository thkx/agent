package worker

import (
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/runtime"
)

type Option func(*Worker)

func WithGraphStore(s runtime.GraphStore) Option {
	return func(e *Worker) {
		e.graphStore = s
	}
}

func WithResultQueue(q *queue.ResultQueue) Option {
	return func(e *Worker) {
		e.resultQueue = q
	}
}

func WithTaskQueue(q *queue.TaskQueue) Option {
	return func(e *Worker) {
		e.taskQueue = q
	}
}
