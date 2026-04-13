package main

import (
	"context"
	"fmt"
	"log"

	"github.com/thkx/agent/agent"
	exampletools "github.com/thkx/agent/examples/tools"
	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/llm/ollama"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/runtime"
	"github.com/thkx/agent/scheduler"
	"github.com/thkx/agent/tracer"
	"github.com/thkx/agent/worker"
)

func main() {
	ctx := context.Background()

	// 创建 memory tracer
	memTracer := tracer.NewMemory()

	taskQ := queue.NewTaskQueue(100)
	resultQ := queue.NewResultQueue(100)

	sched := scheduler.New(taskQ)

	graphStore := runtime.NewMemoryGraphStore()
	rt := runtime.New(
		runtime.WithScheduler(sched),
		runtime.WithResultQueue(resultQ),
		runtime.WithGraphStore(graphStore),
		runtime.WithTracer(memTracer),
	)

	w := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphStore(graphStore),
		worker.WithTracer(memTracer),
	)
	go w.Start(ctx)

	baseLLM := ollama.New(
		llm.WithModel("gemma3:270m"),
		llm.WithBaseURL("http://localhost:11434"),
	)

	ag := agent.New(rt).
		WithLLM(baseLLM).
		WithTools(&exampletools.PriceTool{}).
		WithTracer(memTracer)

	// 运行 agent
	result, err := ag.Run(ctx, "What is the current price of BTC?")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("FINAL ANSWER:", result)

	// 输出 traces
	traces, _ := memTracer.JSON()
	fmt.Println("Traces:", string(traces))
}
