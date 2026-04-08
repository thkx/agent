package queue

import (
	"context"
	"errors"
)

var ErrQueueClosed = errors.New("queue closed")

type Queuer interface {
	Push(context.Context, any) error
	Pop(context.Context) (any, error)
}
