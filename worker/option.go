package worker

import (
	"github.com/thkx/agent/graph"
	"github.com/thkx/agent/queue"
)

type Option func(*Worker)

func WithGraphProvider(f func() *graph.Graph) Option {
	return func(e *Worker) {
		e.graphProvider = f
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
