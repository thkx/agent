package main

import (
	"context"
	"fmt"
	"strings"

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

	taskQ := queue.NewTaskQueue(100)
	resultQ := queue.NewResultQueue(100)

	sched := scheduler.New(taskQ)

	graphStore := runtime.NewMemoryGraphStore()
	rt := runtime.New(
		runtime.WithScheduler(sched),
		runtime.WithResultQueue(resultQ),
		runtime.WithGraphStore(graphStore),
	)

	w := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphStore(graphStore),
	)
	go w.Start(ctx)

	baseLLM := ollama.New(
		llm.WithModel("gemma3:270m"),
		llm.WithBaseURL("http://localhost:11434"),
	)

	stableLLM := &exampleLLM{inner: baseLLM}

	ag := agent.New(rt).
		WithLLM(stableLLM).
		WithTools(&exampletools.PriceTool{})

	// 添加权限到上下文
	ctx = context.WithValue(ctx, "user_permissions", []string{"read"})

	result, err := ag.Run(ctx, "请获取 BTC 价格")
	if err != nil {
		panic(err)
	}

	fmt.Println("FINAL ANSWER:", result)
}

type exampleLLM struct {
	inner llm.LLM
}

func (l *exampleLLM) Generate(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	augmented := make([]llm.Message, 0, len(messages)+1)
	augmented = append(augmented, llm.Message{
		Role: "system",
		Content: "You are a helpful assistant. " +
			"If you need a tool, return only JSON in the form " +
			`{"tool":"tool_name","args":{"key":"value"}}. ` +
			"After receiving a tool result, answer the user directly in plain text and do not return tool JSON again.",
	})
	augmented = append(augmented, messages...)

	resp, err := l.inner.Generate(ctx, augmented)
	if err != nil {
		fmt.Println("OLLAMA ERROR:", err)
		return nil, err
	}

	fmt.Println("OLLAMA RESPONSE:", resp.Content)

	if strings.TrimSpace(resp.Content) == "" {
		if toolResult := latestToolMessage(messages); toolResult != "" {
			resp.Content = "最终答案：" + toolResult
		}
	}

	return resp, nil
}

func latestToolMessage(messages []llm.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "tool" {
			return messages[i].Content
		}
	}
	return ""
}
