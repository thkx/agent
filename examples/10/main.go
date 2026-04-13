package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/thkx/agent/agent"
	"github.com/thkx/agent/config"
	exampletools "github.com/thkx/agent/examples/tools"
	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/llm/ollama"
	"github.com/thkx/agent/runtime"
	"github.com/thkx/agent/scheduler"
	"github.com/thkx/agent/tracer"
	"github.com/thkx/agent/worker"
)

// ExecutionStats 执行统计信息
type ExecutionStats struct {
	ExecutionID    string
	StartTime      time.Time
	EndTime        time.Time
	Duration       time.Duration
	TraceSpanCount int
	Success        bool
	ErrorMessage   string
}

func main() {
	ctx := context.Background()

	separator := "============================================================"

	fmt.Println(separator)
	fmt.Println("Agent Runtime Complete Example")
	fmt.Println(separator)

	// ============================================================
	// 1. 配置管理 - 从配置系统构建完整栈
	// ============================================================
	fmt.Println("\n[1] Configuration Management")
	fmt.Println("Loading config from environment variables...")

	cfg := config.FromEnv()
	if err := cfg.Validate(); err != nil {
		log.Fatal("Config validation failed:", err)
	}
	fmt.Printf("✓ Config validated: Worker concurrency=%d, Retries=%d\n",
		cfg.Worker.Concurrency, cfg.Worker.Retry.MaxRetries)

	// ============================================================
	// 2. 可观测性 - 设置追踪系统
	// ============================================================
	fmt.Println("\n[2] Observability & Tracing")

	memTracerImpl := tracer.NewMemory()
	var memTracer tracer.Tracer = memTracerImpl
	fmt.Println("✓ Memory tracer initialized")

	// ============================================================
	// 3. 构建分布式栈 - Runtime + Worker + Queues
	// ============================================================
	fmt.Println("\n[3] Distributed Stack Setup")

	builder := config.NewBuilder(cfg)
	taskQ, resultQ, err := builder.BuildQueues()
	if err != nil {
		log.Fatal("Failed to build queues:", err)
	}
	fmt.Printf("✓ Queues created (size=%d)\n", cfg.Queue.QueueSize)

	sched := scheduler.New(taskQ)
	graphStore := runtime.NewMemoryGraphStore()

	rt := runtime.New(
		runtime.WithScheduler(sched),
		runtime.WithResultQueue(resultQ),
		runtime.WithGraphStore(graphStore),
		runtime.WithTracer(memTracer),
	)
	fmt.Println("✓ Runtime engine initialized")

	// 启动 Worker
	w := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphStore(graphStore),
		worker.WithConcurrency(cfg.Worker.Concurrency),
		worker.WithTracer(memTracer),
	)
	w.SetRetryStrategy(worker.ExponentialBackoff{
		BaseDelay:  cfg.Worker.Retry.BaseDelay,
		MaxDelay:   cfg.Worker.Retry.MaxDelay,
		MaxRetries: cfg.Worker.Retry.MaxRetries,
	})

	go w.Start(ctx)
	fmt.Printf("✓ Worker started (concurrency=%d)\n", cfg.Worker.Concurrency)

	// ============================================================
	// 4. 插件系统 - 注册多个 LLM 和 Tool
	// ============================================================
	fmt.Println("\n[4] Plugin System - LLM & Tool Registration")

	primeLLM := ollama.New(
		llm.WithModel("gemma3:270m"),
		llm.WithBaseURL("http://localhost:11434"),
	)
	fmt.Println("✓ Primary LLM (Ollama Gemma3) registered")

	fallbackLLM := ollama.New(
		llm.WithModel("llama3.2:1b"),
		llm.WithBaseURL("http://localhost:11434"),
	)
	fmt.Println("✓ Fallback LLM (Ollama Llama3.2) registered")

	priceTool := &exampletools.PriceTool{}
	fmt.Println("✓ Tool: PriceTool registered")

	// ============================================================
	// 5. Agent 构建和配置
	// ============================================================
	fmt.Println("\n[5] Agent Construction")

	ag := agent.New(rt).
		WithLLM(primeLLM).
		WithTools(priceTool).
		WithTracer(memTracer)

	// 注册插件
	ag.RegisterLLM("primary", primeLLM)
	ag.RegisterLLM("fallback", fallbackLLM)
	ag.RegisterTool("price", priceTool)

	fmt.Println("✓ Agent created with plugins:")
	fmt.Println("  - LLMs: primary, fallback")
	fmt.Println("  - Tools: price")

	// ============================================================
	// 6. 执行统计信息收集
	// ============================================================
	fmt.Println("\n[6] Execution with Statistics")

	stats := ExecutionStats{
		ExecutionID: "example-10-" + fmt.Sprintf("%d", time.Now().Unix()),
		StartTime:   time.Now(),
	}

	fmt.Printf("Execution ID: %s\n", stats.ExecutionID)
	fmt.Println("Processing query: 'What is the current price of BTC?'")

	result, err := ag.Run(ctx, "What is the current price of BTC?")

	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)
	stats.Success = err == nil
	if err != nil {
		stats.ErrorMessage = err.Error()
	}

	// ============================================================
	// 7. 结果和追踪分析
	// ============================================================
	fmt.Println("\n[7] Execution Results & Analysis")

	fmt.Printf("\n▸ Agent Response:\n%s\n", result)

	fmt.Printf("\n▸ Execution Statistics:\n")
	fmt.Printf("  Duration:     %v\n", stats.Duration)
	fmt.Printf("  Status:       %s\n", map[bool]string{true: "SUCCESS", false: "FAILED"}[stats.Success])

	if !stats.Success {
		fmt.Printf("  Error:        %s\n", stats.ErrorMessage)
	}

	// 分析追踪数据
	spans := memTracerImpl.Spans()
	stats.TraceSpanCount = len(spans)
	fmt.Printf("  Trace Spans:  %d\n", len(spans))

	if len(spans) > 0 {
		fmt.Println("\n▸ Trace Information:")
		for i, span := range spans {
			fmt.Printf("  [%d] %s\n", i+1, span.Name)
			fmt.Printf("      ExecutionID: %s\n", span.ExecutionID)
			fmt.Printf("      NodeName:    %s\n", span.NodeName)
			fmt.Printf("      PluginName:  %s\n", span.PluginName)
			fmt.Printf("      Duration:    %v\n", span.EndedAt.Sub(span.StartedAt))
			if span.Error != "" {
				fmt.Printf("      Error:       %s\n", span.Error)
			}
		}
	}

	// ============================================================
	// 8. 性能和配置总结
	// ============================================================
	fmt.Println("\n[8] Summary & Configuration")

	fmt.Println("\n▸ Configuration Applied:")
	fmt.Printf("  Runtime:\n    Checkpoint Type:  %s\n", cfg.Runtime.CheckpointType)
	fmt.Printf("    Task Timeout:     %v\n", cfg.Runtime.TaskTimeout)
	fmt.Printf("  Worker:\n    Concurrency:      %d\n", cfg.Worker.Concurrency)
	fmt.Printf("    Max Retries:      %d\n", cfg.Worker.Retry.MaxRetries)
	fmt.Printf("    Base Delay:       %v\n", cfg.Worker.Retry.BaseDelay)
	fmt.Printf("  Queue:\n    Backend Type:     %s\n", cfg.Queue.BackendType)
	fmt.Printf("    Queue Size:       %d\n", cfg.Queue.QueueSize)
	fmt.Printf("  Tracer:\n    Type:             %s\n", cfg.Tracer.Type)

	fmt.Println("\n▸ Execution Summary:")
	fmt.Printf("  Total Duration:   %v\n", stats.Duration)
	fmt.Printf("  Trace Spans:      %d\n", stats.TraceSpanCount)
	fmt.Printf("  Status:           %s\n", map[bool]string{true: "✓ SUCCESS", false: "✗ FAILED"}[stats.Success])

	// ============================================================
	// 9. 功能展示
	// ============================================================
	fmt.Println("\n[9] Enabled Features")
	fmt.Println("  ✓ Plugin Management (LLM/Tool registration)")
	fmt.Println("  ✓ Configuration Management (environment-based)")
	fmt.Println("  ✓ Distributed Tracing (OpenTelemetry-ready)")
	fmt.Println("  ✓ Error Handling & Retry (exponential backoff)")
	fmt.Println("  ✓ Worker Concurrency (configurable)")
	fmt.Println("  ✓ Task Queue (in-memory/hybrid-ready)")
	fmt.Println("  ✓ Observability (span tracking)")

	fmt.Println()
	fmt.Println(separator)
	fmt.Println("Complete Example Execution Finished")
	fmt.Println(separator)
	fmt.Println()
}
