package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/tool"
)

// type ToolNode struct {
// 	tools map[string]tool.Tool
// }

// func (n *ToolNode) Name() string {
// 	return "tool"
// }

// func (n *ToolNode) Execute(ctx context.Context, state *model.State) (*model.State, error) {
// 	tc, ok := state.Meta["tool"].(*llm.ToolCall)
// 	if !ok {
// 		state.Meta["action"] = "done"
// 		return state, nil
// 	}

// 	t, ok := n.tools[tc.Name]
// 	if !ok {
// 		state.Messages = append(state.Messages, model.Message{
// 			Role:    "system",
// 			Content: fmt.Sprintf("Tool %s not found", tc.Name),
// 		})
// 		state.Meta["action"] = "done"
// 		return state, nil
// 	}

// 	// Tool 超时 & 异常捕获
// 	var result any
// 	var err error

// 	done := make(chan struct{})
// 	go func() {
// 		defer func() {
// 			if r := recover(); r != nil {
// 				err = fmt.Errorf("tool panic: %v", r)
// 			}
// 			close(done)
// 		}()
// 		result, err = t.Execute(ctx, tc.Args)
// 	}()

// 	select {
// 	case <-done:
// 		if err != nil {
// 			state.Messages = append(state.Messages, model.Message{
// 				Role:    "system",
// 				Content: fmt.Sprintf("Tool %s failed: %v", tc.Name, err),
// 			})
// 			state.Meta["action"] = "done"
// 		} else {
// 			state.Messages = append(state.Messages, model.Message{
// 				Role:    "tool",
// 				Content: fmt.Sprintf("%v", result),
// 			})
// 			state.Meta["action"] = "llm"
// 		}
// 	case <-time.After(3 * time.Second):
// 		state.Messages = append(state.Messages, model.Message{
// 			Role:    "system",
// 			Content: fmt.Sprintf("Tool %s timed out", tc.Name),
// 		})
// 		state.Meta["action"] = "done"
// 	}

// 	return state, nil
// }

const TOOL_MAX_EXEC = 3

type ToolNode struct {
	tools map[string]tool.Tool
}

func (n *ToolNode) Name() string { return "tool" }

func (n *ToolNode) Execute(ctx context.Context, state *model.State) (*model.State, error) {

	tc, ok := state.Meta["tool"].(*llm.ToolCall)
	if !ok {
		state.Meta["action"] = "done"
		return state, nil
	}

	t, ok := n.tools[tc.Name]
	if !ok {
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: fmt.Sprintf("Tool %s not found", tc.Name),
		})
		state.Meta["action"] = "done"
		return state, nil
	}

	// 节点执行次数限制
	state.Counts[n.Name()]++
	if state.Counts[n.Name()] > TOOL_MAX_EXEC {
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: "Tool node executed too many times, stopping.",
		})
		state.Meta["action"] = "done"
		return state, nil
	}

	// 捕获 panic
	defer func() {
		if r := recover(); r != nil {
			state.Messages = append(state.Messages, model.Message{
				Role:    "system",
				Content: fmt.Sprintf("Tool %s panicked: %v", tc.Name, r),
			})
			state.Meta["action"] = "done"
		}
	}()

	// 超时执行
	ctx2, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	result, err := t.Execute(ctx2, tc.Args)
	if err != nil {
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: fmt.Sprintf("Tool %s failed: %v", tc.Name, err),
		})
		state.Meta["action"] = "done"
		return state, nil
	}

	state.Messages = append(state.Messages, model.Message{
		Role:    "tool",
		Content: fmt.Sprintf("%v", result),
	})

	// 执行完返回 LLM
	state.Meta["action"] = "llm"

	return state, nil
}
