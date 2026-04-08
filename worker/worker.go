package worker

import (
	"context"
	"fmt"

	"github.com/thkx/agent/graph"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/queue"
)

type Worker struct {
	taskQueue     *queue.TaskQueue
	resultQueue   *queue.ResultQueue
	graphProvider func() *graph.Graph // 🔥 动态获取
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
		raw, err := w.taskQueue.Pop(ctx)
		if err != nil {
			fmt.Println(err)
			return
		}
		task := raw.(*model.Task)

		go w.processTask(ctx, task)
		// g := w.graphProvider()
		// if g == nil {
		// 	panic("graph not set")
		// }

		// node, ok := g.GetNode(task.NodeName)
		// if !ok {
		// 	fmt.Printf("Warning: node not found: %s\n", task.NodeName)
		// }

		// newState, err := node.Execute(ctx, task.State)

		// w.resultQueue.Push(ctx, &model.TaskResult{
		// 	ExecutionID: task.ExecutionID,
		// 	NodeName:    task.NodeName,
		// 	State:       newState,
		// 	Error:       err,
		// })
	}
}

func (w *Worker) processTask(ctx context.Context, t *model.Task) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Worker panic:", r)
			w.taskQueue.RetryTask(t)
		}
	}()

	g := w.graphProvider()
	node, ok := g.GetNode(t.NodeName)
	if !ok {
		fmt.Println("Node not found:", t.NodeName)
		return
	}

	newState, err := node.Execute(ctx, t.State)
	if err != nil {
		fmt.Println("Task execution error:", err)
		w.taskQueue.RetryTask(t)
		return
	}

	// 执行成功 ACK
	nextNode, err := g.Next(t.NodeName, newState)
	if err != nil || nextNode == "" {
		_ = w.resultQueue.PushResult(ctx, &model.TaskResult{
			ExecutionID: t.ExecutionID,
			NodeName:    t.NodeName,
			State:       newState,
			Error:       nil,
		})
		return
	}
	_ = w.taskQueue.PushTask(&model.Task{
		ExecutionID: t.ExecutionID,
		NodeName:    nextNode,
		State:       newState,
	})
}
