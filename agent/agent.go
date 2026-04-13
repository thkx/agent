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
	"github.com/thkx/agent/tracer"
)

type Agent struct {
	llm          llm.LLM
	toolRegistry *tool.Registry
	toolRuntime  toolruntime.ToolRuntime
	toolBus      toolbus.Caller
	rt           *runtime.Engine
	allowedTools map[string]bool
	tracer       tracer.Tracer
	// 新增：插件管理
	llmPlugins  map[string]llm.LLM
	toolPlugins map[string]tool.Tool
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
		tracer:       tracer.NewNoop(),
		llmPlugins:   make(map[string]llm.LLM),
		toolPlugins:  make(map[string]tool.Tool),
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
	// 如果是 LocalRuntime，设置 tracer
	if lr, ok := runtime.(*toolruntime.LocalRuntime); ok {
		lr.WithTracer(a.tracer)
	}
	return a
}

func (a *Agent) WithAllowedTools(names ...string) *Agent {
	for _, name := range names {
		a.allowedTools[name] = true
	}
	return a
}

func (a *Agent) WithTracer(t tracer.Tracer) *Agent {
	if t != nil {
		a.tracer = t
	}
	return a
}

func (a *Agent) RegisterLLM(name string, l llm.LLM) {
	a.llmPlugins[name] = l
}

func (a *Agent) RegisterTool(name string, t tool.Tool) {
	a.toolPlugins[name] = t
	a.toolRegistry.Register(t)
	a.allowedTools[t.Name()] = true
}

func (a *Agent) Run(ctx context.Context, input string) (string, error) {
	state := &model.State{
		Messages: []model.Message{
			{Role: "user", Content: input},
		},
		Counts: map[string]int{},
	}

	execID := uuid.New().String()
	ctx, endSpan := a.tracer.StartSpan(ctx, tracer.Span{
		Name:        "agent_run",
		ExecutionID: execID,
		NodeName:    "agent",
		Input:       input,
	})
	defer func() {
		endSpan(state.Messages[len(state.Messages)-1].Content, nil)
	}()

	g := a.buildGraph()

	err := a.rt.Run(ctx, execID, g, state)
	if err != nil {
		endSpan("", err)
		return "", err
	}

	return state.Messages[len(state.Messages)-1].Content, nil
}

func (a *Agent) buildGraph() *graph.Graph {
	g := graph.New("agent", "end")

	rt := NewRuntime(
		NewLLMPlanner(
			a.llm,
			a.toolRegistry,
			a.currentAllowedTools(),
		),
		a.toolBus,
		a.tracer,
	)
	agentNode := &AgentNode{runtime: rt}
	toolNode := &ToolNode{runtime: rt}
	endNode := &EndNode{}

	g.AddNode(agentNode)
	g.AddNode(toolNode)
	g.AddNode(endNode)
	g.AddConditionalEdge("agent", func(state *model.State) (string, bool) {
		switch state.Action {
		case model.ActionTool:
			return "tool", true
		case model.ActionEnd, model.ActionNone:
			return "end", true
		default:
			return "", false
		}
	})
	g.AddConditionalEdge("tool", func(state *model.State) (string, bool) {
		switch state.Action {
		case model.ActionLLM:
			return "agent", true
		case model.ActionEnd, model.ActionNone:
			return "end", true
		default:
			return "", false
		}
	})

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
