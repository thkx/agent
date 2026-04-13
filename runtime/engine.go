package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/thkx/agent/checkpoint"
	"github.com/thkx/agent/graph"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/tracer"
)

type (
	Scheduler interface {
		Submit(context.Context, *model.Task) error
	}

	ExecutionHook interface {
		BeforeExecute(ctx context.Context, task *model.Task) error
		AfterExecute(ctx context.Context, result *model.TaskResult) error
	}
)

type Engine struct {
	graphStore  GraphStore
	scheduler   Scheduler
	checkpoint  checkpoint.Checkpointer
	resultQueue *queue.ResultQueue
	tracer      tracer.Tracer
	resultsMu   sync.Mutex
	pending     map[string][]*model.TaskResult
	hooks       []ExecutionHook // 执行钩子
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

	if e.pending == nil {
		e.pending = make(map[string][]*model.TaskResult)
	}
	if e.tracer == nil {
		e.tracer = tracer.NewNoop()
	}
	if e.hooks == nil {
		e.hooks = []ExecutionHook{}
	}

	return e
}

func (e *Engine) AddHook(h ExecutionHook) {
	e.hooks = append(e.hooks, h)
}

func (e *Engine) Run(ctx context.Context, execID string, g *graph.Graph, init *model.State) error {
	if g == nil {
		return fmt.Errorf("graph required")
	}
	var runErr error
	ctx, finishRun := e.tracer.StartSpan(ctx, tracer.Span{
		Name:        "engine.run",
		ExecutionID: execID,
		Input:       init.Clone(),
	})
	defer func() { finishRun(init.Clone(), runErr) }()

	e.graphStore.Save(execID, g)
	defer e.graphStore.Delete(execID)

	cp := e.checkpoint.Load(ctx, execID)

	current := g.Start()
	state := init

	if cp != nil {
		current = cp.NodeName
		state = cp.State
	}

	task := &model.Task{
		ExecutionID: execID,
		NodeName:    current,
		State:       state,
	}
	_, finishSubmit := e.tracer.StartSpan(ctx, tracer.Span{
		Name:        "engine.submit",
		ExecutionID: execID,
		NodeName:    current,
		Input:       task.Clone(),
	})
	err := e.scheduler.Submit(ctx, task)
	finishSubmit(task.Clone(), err)
	if err != nil {
		runErr = err
		return err
	}

	for {
		res, err := e.awaitResult(ctx, execID)
		if err != nil {
			runErr = err
			return err
		}
		_, finishResult := e.tracer.StartSpan(ctx, tracer.Span{
			Name:        "engine.result",
			ExecutionID: execID,
			NodeName:    res.NodeName,
			Input:       res.Clone(),
		})

		if res.Error != nil {
			finishResult(nil, res.Error)
			runErr = res.Error
			return res.Error
		}

		copyState(init, res.State)
		finishResult(res.Clone(), nil)

		if res.NodeName == g.End() {
			snapshot := &model.ExecutionSnapshot{
				ExecutionID: execID,
				NodeName:    res.NodeName,
				State:       res.State,
			}
			e.checkpoint.Save(ctx, snapshot)
			return nil
		}

		next, err := g.Next(res.NodeName, res.State)
		if err != nil {
			runErr = err
			return err
		}

		_, finishTransition := e.tracer.StartSpan(ctx, tracer.Span{
			Name:        "engine.transition",
			ExecutionID: execID,
			NodeName:    res.NodeName,
			Input:       res.State.Clone(),
		})

		snapshot := &model.ExecutionSnapshot{
			ExecutionID: execID,
			NodeName:    next,
			State:       res.State,
		}
		e.checkpoint.Save(ctx, snapshot)

		nextTask := &model.Task{
			ExecutionID: execID,
			NodeName:    next,
			State:       res.State,
		}
		err = e.scheduler.Submit(ctx, nextTask)
		finishTransition(nextTask.Clone(), err)
		if err != nil {
			runErr = err
			return err
		}
	}
}

func copyState(dst *model.State, src *model.State) {
	if dst == nil || src == nil {
		return
	}

	cloned := src.Clone()
	*dst = *cloned
}

func (e *Engine) awaitResult(ctx context.Context, execID string) (*model.TaskResult, error) {
	// 首先检查待处理队列中是否有此execution的结果
	e.resultsMu.Lock()
	if len(e.pending[execID]) > 0 {
		res := e.pending[execID][0]
		e.pending[execID] = e.pending[execID][1:]
		if len(e.pending[execID]) == 0 {
			delete(e.pending, execID)
		}
		e.resultsMu.Unlock()
		return res, nil
	}
	e.resultsMu.Unlock()

	// 直接从结果队列获取，避免自旋轮询
	for {
		res, err := e.resultQueue.PopResult(ctx)
		if err != nil {
			return nil, err
		}

		if res.ExecutionID == execID {
			return res, nil
		}

		// 结果属于其他execution，存储在待处理队列中
		e.resultsMu.Lock()
		e.pending[res.ExecutionID] = append(e.pending[res.ExecutionID], res)
		e.resultsMu.Unlock()
	}
}
