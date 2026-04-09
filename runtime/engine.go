package runtime

import (
	"context"
	"fmt"

	"github.com/thkx/agent/checkpoint"
	"github.com/thkx/agent/graph"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/queue"
)

type (
	Scheduler interface {
		Submit(context.Context, *model.Task) error
	}
)

type Engine struct {
	graphStore  GraphStore
	scheduler   Scheduler
	checkpoint  checkpoint.Checkpointer
	resultQueue *queue.ResultQueue
	trace       []string // 记录每一步节点执行情况
}

func New(opts ...Option) *Engine {
	e := &Engine{}

	for _, opt := range opts {
		opt(e)
	}

	// ✅ 默认值（关键）
	if e.checkpoint == nil {
		e.checkpoint = checkpoint.NewNoopCheckpoint()
	}

	if e.graphStore == nil {
		e.graphStore = NewMemoryGraphStore()
	}

	if e.scheduler == nil {
		panic("scheduler required")
	}

	if e.resultQueue == nil {
		panic("resultQueue required")
	}

	return e
}

func (e *Engine) logTrace(node string, content string) {
	e.trace = append(e.trace, fmt.Sprintf("%s: %s", node, content))
}

func (e *Engine) Run(ctx context.Context, execID string, g *graph.Graph, init *model.State) error {
	if g == nil {
		return fmt.Errorf("graph required")
	}
	e.graphStore.Save(execID, g)
	defer e.graphStore.Delete(execID)

	cp := e.checkpoint.Load(ctx, execID)

	current := g.Start()
	state := init

	if cp != nil {
		current = cp.NodeName
		state = cp.State
	}

	err := e.scheduler.Submit(ctx, &model.Task{
		ExecutionID: execID,
		NodeName:    current,
		State:       state,
	})
	if err != nil {
		return err
	}

	for {
		res, err := e.resultQueue.PopResult(ctx)
		if err != nil {
			return err
		}

		if res.Error != nil {
			return res.Error
		}

		e.checkpoint.Save(ctx, &model.ExecutionSnapshot{
			ExecutionID: execID,
			NodeName:    res.NodeName,
			State:       res.State,
		})

		if res.NodeName == g.End() {
			return nil
		}

		next, err := g.Next(res.NodeName, res.State)
		if err != nil {
			return err
		}

		if err := e.scheduler.Submit(ctx, &model.Task{
			ExecutionID: execID,
			NodeName:    next,
			State:       res.State,
		}); err != nil {
			return err
		}
	}
}
