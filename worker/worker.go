package worker

import (
	"context"
	"errors"
	"fmt"
	stdruntime "runtime"
	"sync"
	"time"

	"github.com/thkx/agent/model"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/runtime"
	"github.com/thkx/agent/tracer"
)

type RetryStrategy interface {
	ShouldRetry(err error, attempt int) (bool, time.Duration)
}

type ExponentialBackoff struct {
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	MaxRetries int
}

func (e ExponentialBackoff) ShouldRetry(err error, attempt int) (bool, time.Duration) {
	if attempt >= e.MaxRetries {
		return false, 0
	}

	var retryable bool
	if re, ok := err.(interface{ ShouldRetry() bool }); ok {
		retryable = re.ShouldRetry()
	} else {
		// 默认对未知错误也重试
		retryable = true
	}

	if !retryable {
		return false, 0
	}

	delay := e.BaseDelay * time.Duration(1<<attempt)
	if delay > e.MaxDelay {
		delay = e.MaxDelay
	}
	return true, delay
}

type Worker struct {
	taskQueue     *queue.TaskQueue
	resultQueue   *queue.ResultQueue
	graphStore    runtime.GraphStore
	concurrency   int
	tracer        tracer.Tracer
	hooks         []runtime.ExecutionHook // 执行钩子
	retryStrategy RetryStrategy
}

func New(opts ...Option) *Worker {
	e := &Worker{}

	for _, opt := range opts {
		opt(e)
	}

	if e.concurrency <= 0 {
		e.concurrency = stdruntime.NumCPU()
		if e.concurrency < 1 {
			e.concurrency = 1
		}
	}
	if e.tracer == nil {
		e.tracer = tracer.NewNoop()
	}
	if e.hooks == nil {
		e.hooks = []runtime.ExecutionHook{}
	}
	if e.retryStrategy == nil {
		e.retryStrategy = ExponentialBackoff{
			BaseDelay:  100 * time.Millisecond,
			MaxDelay:   10 * time.Second,
			MaxRetries: 3,
		}
	}

	return e
}

func (w *Worker) AddHook(h runtime.ExecutionHook) {
	w.hooks = append(w.hooks, h)
}

func (w *Worker) SetRetryStrategy(rs RetryStrategy) {
	if rs != nil {
		w.retryStrategy = rs
	}
}

func WithHooks(hooks ...runtime.ExecutionHook) Option {
	return func(w *Worker) {
		w.hooks = append(w.hooks, hooks...)
	}
}

func WithRetryStrategy(strategy RetryStrategy) Option {
	return func(w *Worker) {
		w.retryStrategy = strategy
	}
}

func (w *Worker) Start(ctx context.Context) {
	sem := make(chan struct{}, w.concurrency)
	for {
		task, err := w.taskQueue.PopTask(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, queue.ErrQueueClosed) {
				return
			}
			fmt.Println(err)
			return
		}

		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return
		}

		go func() {
			defer func() { <-sem }()
			w.processTask(ctx, task)
		}()
	}
}

func (w *Worker) processTask(ctx context.Context, t *model.Task) {
	var finishOnce sync.Once
	ctx, finish := w.tracer.StartSpan(ctx, tracer.Span{
		Name:        "worker.process_task",
		ExecutionID: t.ExecutionID,
		NodeName:    t.NodeName,
		Input:       t.Clone(),
	})
	endSpan := func(output any, err error) {
		finishOnce.Do(func() {
			finish(output, err)
		})
	}
	defer endSpan(nil, nil)

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Worker panic:", r)
			endSpan(nil, fmt.Errorf("worker panic: %v", r))
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
		endSpan(nil, errors.New("graph not found for execution"))
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
		endSpan(nil, fmt.Errorf("node not found: %s", t.NodeName))
		return
	}

	execState := t.State.Clone()

	// 调用 BeforeExecute 钩子
	for _, h := range w.hooks {
		if err := h.BeforeExecute(ctx, t); err != nil {
			fmt.Println("BeforeExecute hook error:", err)
			endSpan(nil, err)
			_ = w.resultQueue.PushResult(ctx, &model.TaskResult{
				ExecutionID: t.ExecutionID,
				NodeName:    t.NodeName,
				State:       t.State,
				Error:       err,
			})
			return
		}
	}

	newState, err := node.Execute(ctx, execState)

	// 准备结果
	result := &model.TaskResult{
		ExecutionID: t.ExecutionID,
		NodeName:    t.NodeName,
		State:       newState,
		Error:       err,
	}

	// 调用 AfterExecute 钩子
	for _, h := range w.hooks {
		if hookErr := h.AfterExecute(ctx, result); hookErr != nil {
			fmt.Println("AfterExecute hook error:", hookErr)
			// 钩子错误不影响主流程，但可以记录
		}
	}

	if err != nil {
		fmt.Println("Task execution error:", err)
		endSpan(nil, err)

		// 使用重试策略决定是否重试
		shouldRetry, delay := w.retryStrategy.ShouldRetry(err, t.Retry)
		if shouldRetry {
			fmt.Printf("Retrying task %s after %v\n", t.ExecutionID, delay)
			time.Sleep(delay)
			if w.taskQueue.RetryTask(t) {
				return
			}
		}

		_ = w.resultQueue.PushResult(ctx, result)
		return
	}

	_ = w.resultQueue.PushResult(ctx, result)
	endSpan(newState.Clone(), nil)
}
