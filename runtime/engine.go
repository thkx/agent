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
		Schedule(context.Context, *model.Task) error
	}
)

type Engine struct {
	graph       *graph.Graph
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

func (e *Engine) SetGraph(g *graph.Graph) {
	e.graph = g
}

func (e *Engine) GetGraph() *graph.Graph {
	return e.graph
}

func (e *Engine) Run(ctx context.Context, execID string, init *model.State) error {

	cp := e.checkpoint.Load(ctx, execID)

	current := e.graph.Start()
	state := init

	if cp != nil {
		current = cp.NodeName
		state = cp.State
	}

	err := e.scheduler.Schedule(ctx, &model.Task{
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

		e.checkpoint.Save(ctx, execID, res.NodeName, res.State)

		if res.NodeName == e.graph.End() {
			return nil
		}

		next, _ := e.graph.Next(res.NodeName, res.State)

		_ = e.scheduler.Schedule(ctx, &model.Task{
			ExecutionID: execID,
			NodeName:    next,
			State:       res.State,
		})
	}
}
