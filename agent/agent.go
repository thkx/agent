package agent

import (
	"context"

	"github.com/google/uuid"
	"github.com/thkx/agent/graph"
	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/runtime"
	"github.com/thkx/agent/tool"
)

type Agent struct {
	llm   llm.LLM
	tools map[string]tool.Tool
	rt    *runtime.Engine
}

func New(rt *runtime.Engine) *Agent {
	return &Agent{
		tools: make(map[string]tool.Tool),
		rt:    rt,
	}
}

func (a *Agent) WithLLM(l llm.LLM) *Agent {
	a.llm = l
	return a
}

func (a *Agent) WithTools(ts ...tool.Tool) *Agent {
	for _, t := range ts {
		a.tools[t.Name()] = t
	}
	return a
}

func (a *Agent) Run(ctx context.Context, input string) (string, error) {
	state := &model.State{
		Messages: []model.Message{
			{Role: "user", Content: input},
		},
		Meta:   map[string]any{},
		Counts: map[string]int{},
	}

	execID := uuid.New().String()

	a.rt.SetGraph(a.buildGraph())

	err := a.rt.Run(ctx, execID, state)
	if err != nil {
		return "", err
	}

	return state.Messages[len(state.Messages)-1].Content, nil
}

func (a *Agent) buildGraph() *graph.Graph {

	g := graph.New("llm", "end")

	llmNode := &LLMNode{
		llm:          a.llm,
		allowedTools: map[string]bool{"get_price": true},
	}
	toolNode := &ToolNode{tools: a.tools}
	endNode := &EndNode{}

	g.AddNode(llmNode)
	g.AddNode(toolNode)
	g.AddNode(endNode)

	// 条件路由
	g.AddConditionalEdge("llm", func(s *model.State) (string, bool) {
		if s.Meta["action"] == "tool" {
			return "tool", true
		}
		return "end", true
	})

	g.AddEdge("tool", "llm")

	return g
}
