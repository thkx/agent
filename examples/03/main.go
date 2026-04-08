package main

import (
	"context"

	"github.com/thkx/agent/agent"
	"github.com/thkx/agent/graph"
	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/llm/ollama"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/runtime"
	"github.com/thkx/agent/scheduler"
	"github.com/thkx/agent/tool"
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
	rt := runtime.New(
		runtime.WithScheduler(sched),
		runtime.WithResultQueue(resultQ),
	)

	// worker
	w := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphProvider(func() *graph.Graph {
			return rt.GetGraph()
		}),
	)

	go w.Start(ctx)

	// LLM
	llm := ollama.New(
		llm.WithModel("gemma3:270m"),
		llm.WithBaseURL("http://localhost:11434"),
	)

	// Agent
	ag := agent.New(rt).
		WithLLM(llm).
		WithTools(&tool.PriceTool{})

	result, err := ag.Run(ctx, "请获取 BTC 价格")
	if err != nil {
		panic(err)
	}

	println("RESULT:", result)
}
