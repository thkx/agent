package agent

import (
	"context"

	"github.com/google/uuid"
	"github.com/thkx/agent/graph"
	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/runtime"
	"github.com/thkx/agent/tool"
	"github.com/thkx/agent/toolbus"
	"github.com/thkx/agent/toolruntime"
)

type Agent struct {
	llm          llm.LLM
	toolRegistry *tool.Registry
	toolRuntime  toolruntime.ToolRuntime
	toolBus      toolbus.Caller
	rt           *runtime.Engine
	allowedTools map[string]bool
}

func New(rt *runtime.Engine) *Agent {
	registry := tool.NewRegistry()
	localRuntime := toolruntime.NewLocal(registry)
	return &Agent{
		toolRegistry: registry,
		toolRuntime:  localRuntime,
		toolBus:      toolbus.New(localRuntime),
		rt:           rt,
		allowedTools: map[string]bool{},
	}
}

func (a *Agent) WithLLM(l llm.LLM) *Agent {
	a.llm = l
	return a
}

func (a *Agent) WithTools(ts ...tool.Tool) *Agent {
	a.toolRegistry.Register(ts...)
	for _, t := range ts {
		a.allowedTools[t.Name()] = true
	}
	return a
}

func (a *Agent) WithToolRuntime(runtime toolruntime.ToolRuntime) *Agent {
	if runtime == nil {
		return a
	}
	a.toolRuntime = runtime
	a.toolBus = toolbus.New(runtime)
	return a
}

func (a *Agent) WithAllowedTools(names ...string) *Agent {
	for _, name := range names {
		a.allowedTools[name] = true
	}
	return a
}

func (a *Agent) Run(ctx context.Context, input string) (string, error) {
	state := &model.State{
		Messages: []model.Message{
			{Role: "user", Content: input},
		},
		Counts: map[string]int{},
	}

	execID := uuid.New().String()
	g := a.buildGraph()

	err := a.rt.Run(ctx, execID, g, state)
	if err != nil {
		return "", err
	}

	return state.Messages[len(state.Messages)-1].Content, nil
}

func (a *Agent) buildGraph() *graph.Graph {
	g := graph.New("agent", "end")

	agentNode := &AgentNode{
		runtime: NewRuntime(
			NewLLMPlanner(
				a.llm,
				a.toolRegistry,
				a.currentAllowedTools(),
			),
			a.toolBus,
		),
	}
	endNode := &EndNode{}

	g.AddNode(agentNode)
	g.AddNode(endNode)
	g.AddEdge("agent", "end")

	return g
}

func (a *Agent) currentAllowedTools() map[string]bool {
	allowed := make(map[string]bool, len(a.allowedTools))
	for name, ok := range a.allowedTools {
		allowed[name] = ok
	}
	if len(allowed) == 0 {
		for _, name := range a.toolRegistry.Names() {
			allowed[name] = true
		}
	}
	return allowed
}
