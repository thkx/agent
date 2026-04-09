package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/tool"
)

type Planner interface {
	Plan(context.Context, *model.State) (*PlanResult, error)
}

type PlanResult struct {
	Message  model.Message
	Action   model.Action
	ToolCall *llm.ToolCall
}

type LLMPlanner struct {
	llm          llm.LLM
	catalog      tool.Catalog
	allowedTools map[string]bool
}

func NewLLMPlanner(llmClient llm.LLM, catalog tool.Catalog, allowedTools map[string]bool) *LLMPlanner {
	return &LLMPlanner{
		llm:          llmClient,
		catalog:      catalog,
		allowedTools: allowedTools,
	}
}

func (p *LLMPlanner) Plan(ctx context.Context, state *model.State) (*PlanResult, error) {
	msgs := p.buildMessages(state)
	resp, err := p.llm.Generate(ctx, msgs)
	if err != nil {
		return nil, err
	}

	result := &PlanResult{
		Message: model.Message{
			Role:    "assistant",
			Content: resp.Content,
		},
		Action: model.ActionEnd,
	}

	if resp.ToolCall != nil && p.isAllowed(resp.ToolCall.Name) {
		result.Action = model.ActionTool
		result.ToolCall = resp.ToolCall
		return result, nil
	}

	if tc := p.parseToolCall(resp.Content); tc != nil {
		result.Action = model.ActionTool
		result.ToolCall = tc
	}

	return result, nil
}

func (p *LLMPlanner) buildMessages(state *model.State) []llm.Message {
	var msgs []llm.Message
	if catalogPrompt := p.catalogPrompt(); catalogPrompt != "" {
		msgs = append(msgs, llm.Message{
			Role:    "system",
			Content: catalogPrompt,
		})
	}
	msgs = append(msgs, llm.Message{
		Role: "system",
		Content: "If a tool is needed, respond with JSON exactly like " +
			`{"tool":"tool_name","args":{"key":"value"}}. ` +
			"After you receive a tool result, answer the user directly with the final result instead of repeating tool JSON.",
	})
	for _, m := range state.Messages {
		msgs = append(msgs, llm.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return msgs
}

func (p *LLMPlanner) isAllowed(name string) bool {
	if len(p.allowedTools) > 0 {
		return p.allowedTools[name]
	}
	for _, def := range p.catalog.Definitions() {
		if def.Name == name {
			return true
		}
	}
	return false
}

func (p *LLMPlanner) catalogPrompt() string {
	defs := p.availableDefinitions()
	if len(defs) == 0 {
		return ""
	}

	payload := make([]map[string]any, 0, len(defs))
	for _, def := range defs {
		payload = append(payload, map[string]any{
			"name":        def.Name,
			"description": def.Description,
			"schema":      def.Schema.JSON(),
		})
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return "Available tools: " + string(b)
}

func (p *LLMPlanner) availableDefinitions() []tool.Definition {
	if p.catalog == nil {
		return nil
	}

	defs := p.catalog.Definitions()
	if len(p.allowedTools) == 0 {
		return defs
	}

	filtered := make([]tool.Definition, 0, len(defs))
	for _, def := range defs {
		if p.allowedTools[def.Name] {
			filtered = append(filtered, def)
		}
	}
	return filtered
}

func (p *LLMPlanner) parseToolCall(content string) *llm.ToolCall {
	normalized := normalizeJSONContent(content)
	if normalized == "" {
		return nil
	}

	var standard struct {
		Tool string         `json:"tool"`
		Args map[string]any `json:"args"`
	}
	if err := json.Unmarshal([]byte(normalized), &standard); err == nil && standard.Tool != "" && p.isAllowed(standard.Tool) {
		if standard.Args == nil {
			standard.Args = map[string]any{}
		}
		return &llm.ToolCall{Name: standard.Tool, Args: standard.Args}
	}

	var generic map[string]any
	if err := json.Unmarshal([]byte(normalized), &generic); err != nil || len(generic) != 1 {
		return nil
	}

	for name, raw := range generic {
		if !p.isAllowed(name) {
			return nil
		}

		if args, ok := raw.(map[string]any); ok {
			return &llm.ToolCall{Name: name, Args: args}
		}

		argName := p.singleArgumentName(name)
		if argName == "" {
			return nil
		}
		return &llm.ToolCall{
			Name: name,
			Args: map[string]any{argName: raw},
		}
	}

	return nil
}

func normalizeJSONContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" || !strings.Contains(content, "{") {
		return ""
	}

	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) >= 3 && strings.HasPrefix(lines[0], "```") && strings.TrimSpace(lines[len(lines)-1]) == "```" {
			content = strings.Join(lines[1:len(lines)-1], "\n")
			content = strings.TrimSpace(content)
		}
	}

	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return strings.TrimSpace(content[start : end+1])
}

func (p *LLMPlanner) singleArgumentName(toolName string) string {
	for _, def := range p.availableDefinitions() {
		if def.Name != toolName {
			continue
		}
		if len(def.Schema.Required) == 1 {
			return def.Schema.Required[0]
		}
		if len(def.Schema.Properties) == 1 {
			for name := range def.Schema.Properties {
				return name
			}
		}
	}
	return ""
}

func plannerErrorMessage(err error) model.Message {
	return model.Message{
		Role:    "system",
		Content: fmt.Sprintf("LLM error: %v", err),
	}
}
