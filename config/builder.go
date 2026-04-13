package config

import (
	"github.com/thkx/agent/checkpoint"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/runtime"
	"github.com/thkx/agent/scheduler"
	"github.com/thkx/agent/tracer"
	"github.com/thkx/agent/worker"
)

// Builder 用于从配置构建各个组件
type Builder struct {
	cfg *Config
}

// NewBuilder 创建新的 builder
func NewBuilder(cfg *Config) *Builder {
	return &Builder{cfg: cfg}
}

// BuildTracer 根据配置构建 tracer
func (b *Builder) BuildTracer() (tracer.Tracer, error) {
	switch b.cfg.Tracer.Type {
	case "noop":
		return tracer.NewNoop(), nil
	case "memory":
		return tracer.NewMemory(), nil
	default:
		return tracer.NewMemory(), nil
	}
}

// BuildCheckpointer 根据配置构建 checkpointer
func (b *Builder) BuildCheckpointer() (checkpoint.Checkpointer, error) {
	switch b.cfg.Runtime.CheckpointType {
	case "noop":
		return checkpoint.NewNoopCheckpoint(), nil
	case "memory":
		return checkpoint.New(), nil
	default:
		return checkpoint.NewNoopCheckpoint(), nil
	}
}

// BuildQueues 根据配置构建队列
func (b *Builder) BuildQueues() (*queue.TaskQueue, *queue.ResultQueue, error) {
	switch b.cfg.Queue.BackendType {
	case "memory":
		taskQ := queue.NewTaskQueue(b.cfg.Queue.QueueSize)
		resultQ := queue.NewResultQueue(b.cfg.Queue.QueueSize)
		return taskQ, resultQ, nil

	case "hybrid":
		// 混合队列：内存 + 文件系统持久化
		persistence, err := queue.NewFileSystemPersistence("./queue_data")
		if err != nil {
			return nil, nil, err
		}

		// 这里需要创建一个支持持久化的 TaskQueue
		// 暂时使用内存队列
		taskQ := queue.NewTaskQueue(b.cfg.Queue.QueueSize)
		resultQ := queue.NewResultQueue(b.cfg.Queue.QueueSize)

		// 尝试从持久化存储恢复
		_ = persistence.Recover()

		return taskQ, resultQ, nil

	default:
		taskQ := queue.NewTaskQueue(b.cfg.Queue.QueueSize)
		resultQ := queue.NewResultQueue(b.cfg.Queue.QueueSize)
		return taskQ, resultQ, nil
	}
}

// BuildRuntime 根据配置构建 runtime.Engine
func (b *Builder) BuildRuntime() (*runtime.Engine, error) {
	tracer, err := b.BuildTracer()
	if err != nil {
		return nil, err
	}

	checkpoint, err := b.BuildCheckpointer()
	if err != nil {
		return nil, err
	}

	taskQ, resultQ, err := b.BuildQueues()
	if err != nil {
		return nil, err
	}

	sched := scheduler.New(taskQ)
	graphStore := runtime.NewMemoryGraphStore()

	rt := runtime.New(
		runtime.WithScheduler(sched),
		runtime.WithResultQueue(resultQ),
		runtime.WithGraphStore(graphStore),
		runtime.WithCheckpoint(checkpoint),
		runtime.WithTracer(tracer),
	)

	return rt, nil
}

// BuildWorker 根据配置构建 worker
func (b *Builder) BuildWorker(taskQ *queue.TaskQueue, resultQ *queue.ResultQueue, graphStore runtime.GraphStore) (*worker.Worker, error) {
	tracer, err := b.BuildTracer()
	if err != nil {
		return nil, err
	}

	w := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphStore(graphStore),
		worker.WithConcurrency(b.cfg.Worker.Concurrency),
		worker.WithTracer(tracer),
	)

	w.SetRetryStrategy(worker.ExponentialBackoff{
		BaseDelay:  b.cfg.Worker.Retry.BaseDelay,
		MaxDelay:   b.cfg.Worker.Retry.MaxDelay,
		MaxRetries: b.cfg.Worker.Retry.MaxRetries,
	})

	return w, nil
}

// BuildCompleteStack 构建完整的 Runtime + Worker 栈
func (b *Builder) BuildCompleteStack() (*runtime.Engine, *worker.Worker, error) {
	taskQ, resultQ, err := b.BuildQueues()
	if err != nil {
		return nil, nil, err
	}

	tracer, err := b.BuildTracer()
	if err != nil {
		return nil, nil, err
	}

	checkpoint, err := b.BuildCheckpointer()
	if err != nil {
		return nil, nil, err
	}

	sched := scheduler.New(taskQ)
	graphStore := runtime.NewMemoryGraphStore()

	rt := runtime.New(
		runtime.WithScheduler(sched),
		runtime.WithResultQueue(resultQ),
		runtime.WithGraphStore(graphStore),
		runtime.WithCheckpoint(checkpoint),
		runtime.WithTracer(tracer),
	)

	w := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphStore(graphStore),
		worker.WithConcurrency(b.cfg.Worker.Concurrency),
		worker.WithTracer(tracer),
	)

	w.SetRetryStrategy(worker.ExponentialBackoff{
		BaseDelay:  b.cfg.Worker.Retry.BaseDelay,
		MaxDelay:   b.cfg.Worker.Retry.MaxDelay,
		MaxRetries: b.cfg.Worker.Retry.MaxRetries,
	})

	return rt, w, nil
}
