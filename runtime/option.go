package runtime

import (
	"github.com/thkx/agent/checkpoint"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/tracer"
)

type Option func(*Engine)

func WithScheduler(s Scheduler) Option {
	return func(e *Engine) {
		e.scheduler = s
	}
}

func WithResultQueue(q *queue.ResultQueue) Option {
	return func(e *Engine) {
		e.resultQueue = q
	}
}

func WithCheckpoint(c checkpoint.Checkpointer) Option {
	return func(e *Engine) {
		e.checkpoint = c
	}
}

func WithGraphStore(s GraphStore) Option {
	return func(e *Engine) {
		e.graphStore = s
	}
}

func WithTracer(t tracer.Tracer) Option {
	return func(e *Engine) {
		e.tracer = t
	}
}
