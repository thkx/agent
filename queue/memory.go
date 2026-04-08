package queue

import (
	"context"
	"time"
)

type MemoryQueue struct {
	ch chan any
}

func NewMemoryQueue(size int) *MemoryQueue {
	return &MemoryQueue{
		ch: make(chan any, size),
	}
}

func (q *MemoryQueue) Push(ctx context.Context, item any) error {
	select {
	case q.ch <- item:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *MemoryQueue) Pop(ctx context.Context) (any, error) {
	select {
	case item, ok := <-q.ch:
		if !ok {
			return nil, ErrQueueClosed
		}
		return item, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Retry: NACK + 重试机制
func (q *MemoryQueue) Retry(item any, retry int) {
	delay := time.Duration(100*(1<<retry)) * time.Millisecond
	time.AfterFunc(delay, func() {
		_ = q.Push(context.Background(), item)
	})
}
