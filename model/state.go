package model

import "github.com/thkx/agent/llm"

type Message struct {
	Role       string
	Content    string
	ToolName   string
	ToolCallID string
	ToolCall   *llm.ToolCall
	ToolCalls  []llm.ToolCall
	Error      error // 新增：保留错误上下文
}

type Action string

const (
	ActionNone Action = ""
	ActionEnd  Action = "end"
	ActionLLM  Action = "llm"
	ActionTool Action = "tool"
)

const (
	// MaxMessages 限制每个执行保存的消息数，防止内存无限增长
	MaxMessages = 100
)

type State struct {
	Messages         []Message
	Context          map[string]any
	Action           Action
	PendingToolCall  *llm.ToolCall
	PendingToolCalls []llm.ToolCall
	Counts           map[string]int // 每个节点执行次数
	PluginContext    map[string]any // 插件私有上下文
}

func (s *State) Clone() *State {
	if s == nil {
		return nil
	}

	cloned := &State{
		Action: s.Action,
	}

	if len(s.Messages) > 0 {
		cloned.Messages = make([]Message, len(s.Messages))
		for i, msg := range s.Messages {
			cloned.Messages[i] = cloneMessage(msg)
		}
	}

	if s.Context != nil {
		cloned.Context = cloneMap(s.Context)
	}

	if s.PendingToolCall != nil {
		cloned.PendingToolCall = &llm.ToolCall{
			Name: s.PendingToolCall.Name,
		}
		if s.PendingToolCall.Args != nil {
			cloned.PendingToolCall.Args = cloneMap(s.PendingToolCall.Args)
		}
	}
	if len(s.PendingToolCalls) > 0 {
		cloned.PendingToolCalls = make([]llm.ToolCall, len(s.PendingToolCalls))
		for i, call := range s.PendingToolCalls {
			cloned.PendingToolCalls[i] = cloneLLMToolCall(call)
		}
	}

	if s.Counts != nil {
		cloned.Counts = make(map[string]int, len(s.Counts))
		for k, v := range s.Counts {
			cloned.Counts[k] = v
		}
	}

	if s.PluginContext != nil {
		cloned.PluginContext = cloneMap(s.PluginContext)
	}

	return cloned
}

// TrimMessages 保持消息在设定的最大数量内，保留最新的消息
func (s *State) TrimMessages() {
	if len(s.Messages) > MaxMessages {
		// 保留最后 MaxMessages 条消息
		s.Messages = s.Messages[len(s.Messages)-MaxMessages:]
	}
}

func cloneMessage(msg Message) Message {
	cloned := Message{
		Role:       msg.Role,
		Content:    msg.Content,
		ToolName:   msg.ToolName,
		ToolCallID: msg.ToolCallID,
		Error:      msg.Error, // 克隆错误
	}
	if msg.ToolCall != nil {
		copy := cloneLLMToolCall(*msg.ToolCall)
		cloned.ToolCall = &copy
	}
	if len(msg.ToolCalls) > 0 {
		cloned.ToolCalls = make([]llm.ToolCall, len(msg.ToolCalls))
		for i, call := range msg.ToolCalls {
			cloned.ToolCalls[i] = cloneLLMToolCall(call)
		}
	}
	return cloned
}

func cloneLLMToolCall(call llm.ToolCall) llm.ToolCall {
	cloned := llm.ToolCall{
		ID:   call.ID,
		Name: call.Name,
	}
	if call.Args != nil {
		cloned.Args = cloneMap(call.Args)
	}
	return cloned
}

func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = cloneValue(v)
	}
	return dst
}

func cloneSlice(src []any) []any {
	dst := make([]any, len(src))
	for i, v := range src {
		dst[i] = cloneValue(v)
	}
	return dst
}

func cloneValue(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		return cloneSlice(typed)
	case []string:
		return append([]string(nil), typed...)
	case []int:
		return append([]int(nil), typed...)
	case []float64:
		return append([]float64(nil), typed...)
	case []bool:
		return append([]bool(nil), typed...)
	default:
		return v
	}
}
