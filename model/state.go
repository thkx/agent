package model

import "github.com/thkx/agent/llm"

type Message struct {
	Role    string
	Content string
}

type Action string

const (
	ActionNone Action = ""
	ActionEnd  Action = "end"
	ActionLLM  Action = "llm"
	ActionTool Action = "tool"
)

type State struct {
	Messages        []Message
	Context         map[string]any
	Action          Action
	PendingToolCall *llm.ToolCall
	Counts          map[string]int // 每个节点执行次数
}
