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
)

func main() {
	ctx := context.Background()

	// 使用配置系统构建 Agent
	// 方式 1: 使用默认配置
	cfg := config.Default()
	fmt.Println("Default Config:", cfg)

	// 方式 2: 从环境变量加载配置
	cfgFromEnv := config.FromEnv()
	fmt.Println("Config from Env:", cfgFromEnv)

	// 验证配置
	if err := cfg.Validate(); err != nil {
		log.Fatal("Config validation failed:", err)
	}

	// 使用 builder 构建完整栈
	builder := config.NewBuilder(cfg)
	rt, w, err := builder.BuildCompleteStack()
	if err != nil {
		log.Fatal("Failed to build stack:", err)
	}

	// 启动 worker
	go w.Start(ctx)

	// 创建 agent
	agentCfg, err := builder.BuildTracer()
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
		WithTracer(agentCfg)

	// 运行 agent
	result, err := ag.Run(ctx, "What is the current price of BTC?")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("FINAL ANSWER:", result)
}
