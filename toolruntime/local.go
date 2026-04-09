package toolruntime

import (
	"context"
	"fmt"

	"github.com/thkx/agent/tool"
)

type LocalRuntime struct {
	registry *tool.Registry
}

func NewLocal(registry *tool.Registry) *LocalRuntime {
	if registry == nil {
		registry = tool.NewRegistry()
	}
	return &LocalRuntime{
		registry: registry,
	}
}

func (r *LocalRuntime) Execute(ctx context.Context, call ToolCall) (ToolResult, error) {
	t, ok := r.registry.Get(call.Name)
	if !ok {
		return ToolResult{}, fmt.Errorf("tool %s not found", call.Name)
	}
	if err := t.Schema().Validate(call.Input); err != nil {
		return ToolResult{}, err
	}
	output, err := t.Invoke(ctx, call.Input)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Output: output}, nil
}
