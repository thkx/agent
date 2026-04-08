package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/model"
)

const MAX_NODE_EXEC = 5

type LLMNode struct {
	llm          llm.LLM
	allowedTools map[string]bool // 白名单工具
}

func (n *LLMNode) Name() string { return "llm" }

func (n *LLMNode) Execute(ctx context.Context, state *model.State) (*model.State, error) {
	// 节点执行计数
	if state.Counts == nil {
		state.Counts = make(map[string]int)
	}
	state.Counts[n.Name()]++
	if state.Counts[n.Name()] > MAX_NODE_EXEC {
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: "LLM node executed too many times, stopping.",
		})
		state.Meta["action"] = "done"
		return state, nil
	}

	// 转换消息
	var msgs []llm.Message
	for _, m := range state.Messages {
		msgs = append(msgs, llm.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	resp, err := n.llm.Generate(ctx, msgs)
	if err != nil {
		state.Messages = append(state.Messages, model.Message{
			Role:    "system",
			Content: fmt.Sprintf("LLM error: %v", err),
		})
		state.Meta["action"] = "done"
		return state, nil
	}

	content := resp.Content

	// 尝试解析 ToolCall
	var tc struct {
		Tool string         `json:"tool"`
		Args map[string]any `json:"args"`
	}
	if strings.Contains(content, "{") {
		_ = json.Unmarshal([]byte(content), &tc)
	}

	if tc.Tool != "" && n.allowedTools[tc.Tool] {
		state.Meta["action"] = "tool"
		state.Meta["tool"] = &llm.ToolCall{Name: tc.Tool, Args: tc.Args}
	} else {
		state.Meta["action"] = "done"
	}

	fmt.Println(state)

	state.Messages = append(state.Messages, model.Message{
		Role:    "assistant",
		Content: content,
	})

	return state, nil
}
