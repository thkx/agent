package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thkx/agent/agent"
	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/llm/ollama"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/runtime"
	"github.com/thkx/agent/scheduler"
	"github.com/thkx/agent/tool"
	"github.com/thkx/agent/tracer"
	"github.com/thkx/agent/worker"
)

func main() {
	ctx := context.Background()

	taskQ := queue.NewTaskQueue(100)
	resultQ := queue.NewResultQueue(100)
	graphStore := runtime.NewMemoryGraphStore()
	memTracer := tracer.NewMemory()

	rt := runtime.New(
		runtime.WithScheduler(scheduler.New(taskQ)),
		runtime.WithResultQueue(resultQ),
		runtime.WithGraphStore(graphStore),
		runtime.WithTracer(memTracer),
	)

	w := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphStore(graphStore),
		worker.WithTracer(memTracer),
		worker.WithConcurrency(2),
	)
	go w.Start(ctx)

	baseURL := envOrDefault("OLLAMA_BASE_URL", "http://localhost:11434")
	modelName := envOrDefault("OLLAMA_MODEL", "gemma3:270m")
	tracePath := envOrDefault("TRACE_PATH", "examples/05/trace.json")

	baseLLM := ollama.New(
		llm.WithModel(modelName),
		llm.WithBaseURL(baseURL),
	)

	ag := agent.New(rt).
		WithTracer(memTracer).
		WithLLM(&loggingLLM{inner: baseLLM}).
		WithTools(&PriceTool{}, &VolumeTool{})

	result, err := ag.Run(ctx, "请同时获取 BTC 和 ETH 的价格与成交量")
	if err != nil {
		panic(err)
	}

	fmt.Println("FINAL ANSWER:")
	fmt.Println(result)

	traceJSON, err := memTracer.JSON()
	if err != nil {
		panic(err)
	}

	if err := os.MkdirAll(filepath.Dir(tracePath), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(tracePath, traceJSON, 0o644); err != nil {
		panic(err)
	}

	fmt.Printf("\nTRACE FILE: %s\n", tracePath)
	fmt.Println("\nTRACE JSON:")
	// fmt.Println(string(traceJSON))
}

type loggingLLM struct {
	inner llm.LLM
}

func (l *loggingLLM) Generate(ctx context.Context, messages []llm.Message) (*llm.Response, error) {
	augmented := appendStructuredSystemPrompt(messages)
	resp, err := l.inner.Generate(ctx, augmented)
	if err != nil {
		fmt.Println("OLLAMA ERROR:", err)
		return nil, err
	}
	logLLMResponse(resp)
	return resp, nil
}

func (l *loggingLLM) GenerateWithTools(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition) (*llm.Response, error) {
	caller, ok := l.inner.(llm.StructuredToolCaller)
	if !ok {
		return l.generateWithJSONFallback(ctx, messages, tools)
	}

	augmented := appendStructuredSystemPrompt(messages)
	resp, err := caller.GenerateWithTools(ctx, augmented, tools)
	if err != nil {
		if supportsToolsError(err) {
			fmt.Println("OLLAMA WARNING: model does not support native tools, falling back to JSON tool protocol")
			return l.generateWithJSONFallback(ctx, messages, tools)
		}
		fmt.Println("OLLAMA ERROR:", err)
		return nil, err
	}
	logLLMResponse(resp)
	return resp, nil
}

func (l *loggingLLM) generateWithJSONFallback(ctx context.Context, messages []llm.Message, tools []llm.ToolDefinition) (*llm.Response, error) {
	augmented := appendJSONFallbackPrompt(messages, tools)
	resp, err := l.inner.Generate(ctx, augmented)
	if err != nil {
		fmt.Println("OLLAMA ERROR:", err)
		return nil, err
	}
	logLLMResponse(resp)
	return resp, nil
}

func appendStructuredSystemPrompt(messages []llm.Message) []llm.Message {
	augmented := make([]llm.Message, 0, len(messages)+1)
	augmented = append(augmented, llm.Message{
		Role: "system",
		Content: "You are a helpful assistant. " +
			"When tools are available, prefer native structured tool calls instead of raw JSON. " +
			"If the user asks for multiple facts, you may call multiple tools in one turn. " +
			"After receiving tool results, answer directly in plain text and reference the useful results clearly.",
	})
	augmented = append(augmented, messages...)
	return augmented
}

func appendJSONFallbackPrompt(messages []llm.Message, tools []llm.ToolDefinition) []llm.Message {
	toolPayload := make([]map[string]any, 0, len(tools))
	for _, def := range tools {
		toolPayload = append(toolPayload, map[string]any{
			"name":        def.Name,
			"description": def.Description,
			"schema":      def.Parameters,
		})
	}

	toolJSON, err := json.Marshal(toolPayload)
	if err != nil {
		toolJSON = []byte("[]")
	}

	augmented := make([]llm.Message, 0, len(messages)+2)
	augmented = append(augmented, llm.Message{
		Role:    "system",
		Content: "Available tools: " + string(toolJSON),
	})
	augmented = append(augmented, llm.Message{
		Role: "system",
		Content: "If a tool is needed, respond with JSON exactly like " +
			`{"tool":"tool_name","args":{"key":"value"}}. ` +
			"If multiple tools are needed, respond with JSON exactly like " +
			`{"tool_calls":[{"tool":"tool_name","args":{"key":"value"}},{"tool":"another_tool","args":{"key":"value"}}]}. ` +
			"After receiving tool results, answer directly in plain text.",
	})
	augmented = append(augmented, messages...)
	return augmented
}

func logLLMResponse(resp *llm.Response) {
	if resp == nil {
		fmt.Println("OLLAMA RESPONSE: <nil>")
		return
	}

	if len(resp.ToolCalls) > 0 {
		fmt.Printf("OLLAMA TOOL CALLS: %#v\n", resp.ToolCalls)
		return
	}

	fmt.Println("OLLAMA RESPONSE:", resp.Content)
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func supportsToolsError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not support tools")
}

type PriceTool struct{}

func (t *PriceTool) Name() string { return "get_price" }

func (t *PriceTool) Description() string { return "Get the latest price for a crypto symbol." }

func (t *PriceTool) Schema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Property{
			"symbol": {Type: "string"},
		},
		Required: []string{"symbol"},
	}
}

func (t *PriceTool) Invoke(ctx context.Context, input any) (any, error) {
	args, _ := input.(map[string]any)
	symbol, _ := args["symbol"].(string)
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		symbol = "BTC"
	}
	return fmt.Sprintf("%s price is 65000 (mock)", symbol), nil
}

func (t *PriceTool) Timeout() time.Duration {
	return 10 * time.Second
}

func (t *PriceTool) Permissions() []string {
	return []string{"read"}
}

type VolumeTool struct{}

func (t *VolumeTool) Name() string { return "get_volume" }

func (t *VolumeTool) Description() string { return "Get the latest volume for a crypto symbol." }

func (t *VolumeTool) Schema() tool.Schema {
	return tool.Schema{
		Type: "object",
		Properties: map[string]tool.Property{
			"symbol": {Type: "string"},
		},
		Required: []string{"symbol"},
	}
}

func (t *VolumeTool) Invoke(ctx context.Context, input any) (any, error) {
	args, _ := input.(map[string]any)
	symbol, _ := args["symbol"].(string)
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		symbol = "ETH"
	}
	return fmt.Sprintf("%s volume is 1200000 (mock)", symbol), nil
}

func (t *VolumeTool) Timeout() time.Duration {
	return 10 * time.Second
}

func (t *VolumeTool) Permissions() []string {
	return []string{"read"}
}
