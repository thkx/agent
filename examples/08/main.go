package main

import (
	"context"
	"fmt"
	"log"

	"github.com/thkx/agent/agent"
	exampletools "github.com/thkx/agent/examples/tools"
	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/llm/ollama"
	"github.com/thkx/agent/config"
	"github.com/thkx/agent/queue"
)

func main() {
	ctx := context.Background()

	// 演示：使用持久化队列
	fmt.Println("=== Example 08: Persistent Queue ===")

	// 创建持久化配置
	cfg := config.Default()
	cfg.Queue.BackendType = "hybrid"

	// 演示文件系统持久化
	persistence, err := queue.NewFileSystemPersistence("./queue_cache")
	if err != nil {
		log.Fatal("Failed to create persistence:", err)
	}
	fmt.Println("✓ Created filesystem persistence at ./queue_cache")

	// 使用配置构建栈
	builder := config.NewBuilder(cfg)
	rt, w, err := builder.BuildCompleteStack()
	if err != nil {
		log.Fatal("Failed to build stack:", err)
	}
	fmt.Println("✓ Built complete runtime stack with hybrid queues")

	// 启动 worker
	go w.Start(ctx)
	fmt.Println("✓ Started worker")

	// 创建 agent
	tracer, err := builder.BuildTracer()
	if err != nil {
		log.Fatal("Failed to build tracer:", err)
	}

	baseLLM := ollama.New(
		llm.WithModel("gemma3:270m"),
		llm.WithBaseURL("http://localhost:11434"),
	)

	ag := agent.New(rt).
		WithLLM(baseLLM).
		WithTools(&exampletools.PriceTool{}).
		WithTracer(tracer)

	fmt.Println("✓ Created agent with persistent queue support")

	// 运行 agent
	result, err := ag.Run(ctx, "What is the current price of BTC?")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nFINAL ANSWER:", result)
	fmt.Println("\n✓ Execution completed with persistent queue support")

	// 持久化队列数据（演示）
	if err := persistence.Persist(); err != nil {
		fmt.Println("Warning: Failed to persist queue data:", err)
	} else {
		fmt.Println("✓ Queue data persisted to disk")
	}
}
