package toolruntime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/thkx/agent/model"
	"github.com/thkx/agent/tool"
	"github.com/thkx/agent/tracer"
)

type LocalRuntime struct {
	registry *tool.Registry
	tracer   tracer.Tracer
}

func NewLocal(registry *tool.Registry) *LocalRuntime {
	if registry == nil {
		registry = tool.NewRegistry()
	}
	return &LocalRuntime{
		registry: registry,
		tracer:   tracer.NewNoop(),
	}
}

func (r *LocalRuntime) WithTracer(t tracer.Tracer) *LocalRuntime {
	if t != nil {
		r.tracer = t
	}
	return r
}

func (r *LocalRuntime) Execute(ctx context.Context, call ToolCall) (ToolResult, error) {
	ctx, endSpan := r.tracer.StartSpan(ctx, tracer.Span{
		Name:        "tool.execute",
		ExecutionID: "", // 需要从上下文获取？
		NodeName:    "tool",
		PluginName:  call.Name,
		Input:       call.Input,
	})
	defer func() {
		// endSpan will be called with output or error
	}()

	t, ok := r.registry.Get(call.Name)
	if !ok {
		err := model.FatalError{Err: fmt.Errorf("tool %s not found", call.Name)}
		endSpan(nil, err)
		return ToolResult{}, err
	}

	// 检查权限（示例：假设上下文中有用户权限）
	if !r.checkPermissions(ctx, t.Permissions()) {
		err := model.FatalError{Err: errors.New("permission denied")}
		endSpan(nil, err)
		return ToolResult{}, err
	}

	if err := t.Schema().Validate(call.Input); err != nil {
		err := model.FatalError{Err: err}
		endSpan(nil, err)
		return ToolResult{}, err
	}

	// 应用超时
	timeoutCtx, cancel := context.WithTimeout(ctx, t.Timeout())
	defer cancel()

	output, err := t.Invoke(timeoutCtx, call.Input)
	if err != nil {
		// 根据错误类型返回可重试或致命错误
		if errors.Is(err, context.DeadlineExceeded) {
			retryErr := model.RetryableError{Err: err, RetryAfter: time.Second}
			endSpan(nil, retryErr)
			return ToolResult{}, retryErr
		}
		return ToolResult{}, model.FatalError{Err: err}
	}
	endSpan(output, nil)
	return ToolResult{Output: output}, nil
}

func (r *LocalRuntime) checkPermissions(ctx context.Context, required []string) bool {
	// 临时：总是允许
	return true
}
