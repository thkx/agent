package toolbus

import (
	"context"
	"log"

	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/toolruntime"
)

type Caller interface {
	Call(context.Context, *llm.ToolCall) (any, error)
}

type ToolBus struct {
	runtime toolruntime.ToolRuntime
}

func New(runtime toolruntime.ToolRuntime) *ToolBus {
	return &ToolBus{runtime: runtime}
}

func (b *ToolBus) Call(ctx context.Context, call *llm.ToolCall) (any, error) {
	log.Println("ToolBus Call")
	result, err := b.runtime.Execute(ctx, toolruntime.ToolCall{
		Name:  call.Name,
		Input: call.Args,
	})
	if err != nil {
		return nil, err
	}
	return result.Output, nil
}
