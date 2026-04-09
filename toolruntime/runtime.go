package toolruntime

import "context"

type ToolCall struct {
	Name  string
	Input any
}

type ToolResult struct {
	Output any
}

type ToolRuntime interface {
	Execute(context.Context, ToolCall) (ToolResult, error)
}
