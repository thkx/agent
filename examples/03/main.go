package main

import (
	"context"
	"fmt"

	"github.com/thkx/agent/agent"
	exampletools "github.com/thkx/agent/examples/tools"
	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/llm/ollama"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/runtime"
	"github.com/thkx/agent/scheduler"
	"github.com/thkx/agent/worker"
)

func main() {

	ctx := context.Background()

	// 队列
	taskQ := queue.NewTaskQueue(100)
	resultQ := queue.NewResultQueue(100)

	// scheduler
	sched := scheduler.New(taskQ)

	// runtime
	graphStore := runtime.NewMemoryGraphStore()
	rt := runtime.New(
		runtime.WithScheduler(sched),
		runtime.WithResultQueue(resultQ),
		runtime.WithGraphStore(graphStore),
	)

	// worker
	w := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphStore(graphStore),
	)

	go w.Start(ctx)

	// LLM
	baseLLM := ollama.New(
		llm.WithModel("gemma3:270m"),
		llm.WithBaseURL("http://localhost:11434"),
	)
	debugLLM := &loggingLLM{inner: baseLLM}

	// Agent
	ag := agent.New(rt).
		WithLLM(debugLLM).
		WithTools(&exampletools.PriceTool{})

	result, err := ag.Run(ctx, "请获取 BTC 价格")
	if err != nil {
		panic(err)
	}

	println("RESULT:", result)
}

type loggingLLM struct {
	inner llm.LLM
}

func (l *loggingLLM) Generate(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	resp, err := l.inner.Generate(ctx, messages)
	if err != nil {
		fmt.Println("OLLAMA ERROR:", err)
		return nil, err
	}
	fmt.Println("OLLAMA RESPONSE:", resp.Content)
	return resp, nil
}
