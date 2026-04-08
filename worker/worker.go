package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/thkx/agent/model"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/runtime"
)

type Worker struct {
	taskQueue   *queue.TaskQueue
	resultQueue *queue.ResultQueue
	graphStore  runtime.GraphStore
}

func New(opts ...Option) *Worker {
	e := &Worker{}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

func (w *Worker) Start(ctx context.Context) {
	for {
		task, err := w.taskQueue.PopTask(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, queue.ErrQueueClosed) {
				return
			}
			fmt.Println(err)
			return
		}

		go w.processTask(ctx, task)
	}
}

func (w *Worker) processTask(ctx context.Context, t *model.Task) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Worker panic:", r)
			if !w.taskQueue.RetryTask(t) {
				_ = w.resultQueue.PushResult(ctx, &model.TaskResult{
					ExecutionID: t.ExecutionID,
					NodeName:    t.NodeName,
					State:       t.State,
					Error:       fmt.Errorf("worker panic: %v", r),
				})
			}
		}
	}()

	g, ok := w.graphStore.Load(t.ExecutionID)
	if !ok || g == nil {
		_ = w.resultQueue.PushResult(ctx, &model.TaskResult{
			ExecutionID: t.ExecutionID,
			NodeName:    t.NodeName,
			State:       t.State,
			Error:       errors.New("graph not found for execution"),
		})
		return
	}

	node, ok := g.GetNode(t.NodeName)
	if !ok {
		_ = w.resultQueue.PushResult(ctx, &model.TaskResult{
			ExecutionID: t.ExecutionID,
			NodeName:    t.NodeName,
			State:       t.State,
			Error:       fmt.Errorf("node not found: %s", t.NodeName),
		})
		return
	}

	newState, err := node.Execute(ctx, t.State)
	if err != nil {
		fmt.Println("Task execution error:", err)
		if w.taskQueue.RetryTask(t) {
			return
		}

		_ = w.resultQueue.PushResult(ctx, &model.TaskResult{
			ExecutionID: t.ExecutionID,
			NodeName:    t.NodeName,
			State:       t.State,
			Error:       err,
		})
		return
	}

	_ = w.resultQueue.PushResult(ctx, &model.TaskResult{
		ExecutionID: t.ExecutionID,
		NodeName:    t.NodeName,
		State:       newState,
	})
}
