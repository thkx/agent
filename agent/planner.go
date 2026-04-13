package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/thkx/agent/llm"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/tool"
)

type Planner interface {
	Plan(context.Context, *model.State) (*PlanResult, error)
}

type PlanResult struct {
	Message   model.Message
	Action    model.Action
	ToolCall  *llm.ToolCall
	ToolCalls []llm.ToolCall
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
	structured := p.supportsStructuredTools()
	msgs := p.buildMessages(state, structured)
	resp, err := p.generate(ctx, msgs)
	if err != nil {
		return nil, err
	}

	result := &PlanResult{
		Message: model.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCall:  cloneToolCall(resp.ToolCall),
			ToolCalls: cloneToolCalls(resp.ToolCalls),
		},
		Action: model.ActionEnd,
	}

	if len(resp.ToolCalls) > 0 {
		allowed := filterAllowedToolCalls(resp.ToolCalls, p.isAllowed)
		ensureToolCallIDs(allowed)
		if len(allowed) > 0 {
			result.Action = model.ActionTool
			result.ToolCalls = allowed
			result.ToolCall = cloneToolCallPtrFromSlice(allowed)
			result.Message.ToolCalls = cloneToolCalls(allowed)
			result.Message.ToolCall = cloneToolCallPtrFromSlice(allowed)
			return result, nil
		}
	}

	if resp.ToolCall != nil && p.isAllowed(resp.ToolCall.Name) {
		ensureToolCallID(resp.ToolCall)
		result.Action = model.ActionTool
		result.ToolCall = resp.ToolCall
		result.ToolCalls = []llm.ToolCall{*cloneToolCall(resp.ToolCall)}
		result.Message.ToolCall = cloneToolCall(resp.ToolCall)
		result.Message.ToolCalls = cloneToolCalls(result.ToolCalls)
		return result, nil
	}

	if toolCalls := p.parseToolCalls(resp.Content); len(toolCalls) > 0 {
		result.Action = model.ActionTool
		result.ToolCalls = toolCalls
		result.ToolCall = cloneToolCallPtrFromSlice(toolCalls)
		result.Message.ToolCalls = cloneToolCalls(toolCalls)
		result.Message.ToolCall = cloneToolCallPtrFromSlice(toolCalls)
		return result, nil
	}

	if tc := p.parseToolCall(resp.Content); tc != nil {
		ensureToolCallID(tc)
		result.Action = model.ActionTool
		result.ToolCall = tc
		result.ToolCalls = []llm.ToolCall{*cloneToolCall(tc)}
		result.Message.ToolCall = cloneToolCall(tc)
		result.Message.ToolCalls = cloneToolCalls(result.ToolCalls)
	}

	return result, nil
}

func (p *LLMPlanner) buildMessages(state *model.State, structured bool) []llm.Message {
	var msgs []llm.Message
	if !structured {
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
	} else {
		msgs = append(msgs, llm.Message{
			Role:    "system",
			Content: "Use native tool calls when a tool is needed. After you receive a tool result, answer the user directly in plain text.",
		})
	}
	for _, m := range state.Messages {
		msg := llm.Message{
			Role:       m.Role,
			Content:    m.Content,
			ToolName:   m.ToolName,
			ToolCallID: m.ToolCallID,
		}
		if m.ToolCall != nil {
			msg.ToolCalls = []llm.ToolCall{*cloneToolCall(m.ToolCall)}
		}
		if len(m.ToolCalls) > 0 {
			msg.ToolCalls = cloneToolCalls(m.ToolCalls)
		}
		msgs = append(msgs, msg)
	}
	return msgs
}

func (p *LLMPlanner) generate(ctx context.Context, msgs []llm.Message) (*llm.Response, error) {
	if caller, ok := p.llm.(llm.StructuredToolCaller); ok {
		defs := p.availableDefinitions()
		if len(defs) > 0 {
			return caller.GenerateWithTools(ctx, msgs, p.toLLMTools(defs))
		}
	}
	return p.llm.Generate(ctx, msgs)
}

func (p *LLMPlanner) supportsStructuredTools() bool {
	caller, ok := p.llm.(llm.StructuredToolCaller)
	return ok && caller != nil && len(p.availableDefinitions()) > 0
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

func (p *LLMPlanner) toLLMTools(defs []tool.Definition) []llm.ToolDefinition {
	out := make([]llm.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		out = append(out, llm.ToolDefinition{
			Name:        def.Name,
			Description: def.Description,
			Parameters:  def.Schema.JSON(),
		})
	}
	return out
}

func (p *LLMPlanner) parseToolCalls(content string) []llm.ToolCall {
	normalized := normalizeJSONContent(content)
	if normalized == "" {
		return nil
	}

	var payload struct {
		ToolCalls []struct {
			Tool string         `json:"tool"`
			Args map[string]any `json:"args"`
		} `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(normalized), &payload); err != nil || len(payload.ToolCalls) == 0 {
		return nil
	}

	parsed := make([]llm.ToolCall, 0, len(payload.ToolCalls))
	for _, item := range payload.ToolCalls {
		if item.Tool == "" || !p.isAllowed(item.Tool) {
			return nil
		}
		if item.Args == nil {
			item.Args = map[string]any{}
		}
		parsed = append(parsed, llm.ToolCall{Name: item.Tool, Args: item.Args})
	}
	ensureToolCallIDs(parsed)
	return parsed
}

func (p *LLMPlanner) parseToolCall(content string) *llm.ToolCall {
	normalized := normalizeJSONContent(content)
	if normalized == "" {
		return nil
	}

	var standard struct {
		Tool      string         `json:"tool"`
		Args      map[string]any `json:"args"`
		ToolCalls []struct {
			Tool string         `json:"tool"`
			Args map[string]any `json:"args"`
		} `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(normalized), &standard); err == nil {
		if standard.Tool != "" && p.isAllowed(standard.Tool) {
			if standard.Args == nil {
				standard.Args = map[string]any{}
			}
			call := &llm.ToolCall{Name: standard.Tool, Args: standard.Args}
			ensureToolCallID(call)
			return call
		}
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

func cloneToolCall(call *llm.ToolCall) *llm.ToolCall {
	if call == nil {
		return nil
	}

	cloned := &llm.ToolCall{
		ID:   call.ID,
		Name: call.Name,
	}
	if call.Args != nil {
		cloned.Args = make(map[string]any, len(call.Args))
		for k, v := range call.Args {
			cloned.Args[k] = v
		}
	}
	return cloned
}

func cloneToolCalls(calls []llm.ToolCall) []llm.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	cloned := make([]llm.ToolCall, len(calls))
	for i, call := range calls {
		cloned[i] = *cloneToolCall(&call)
	}
	return cloned
}

func cloneToolCallPtrFromSlice(calls []llm.ToolCall) *llm.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	return cloneToolCall(&calls[0])
}

func filterAllowedToolCalls(calls []llm.ToolCall, isAllowed func(string) bool) []llm.ToolCall {
	filtered := make([]llm.ToolCall, 0, len(calls))
	for _, call := range calls {
		if !isAllowed(call.Name) {
			continue
		}
		filtered = append(filtered, *cloneToolCall(&call))
	}
	return filtered
}

func ensureToolCallIDs(calls []llm.ToolCall) {
	for i := range calls {
		if calls[i].ID == "" {
			calls[i].ID = "call_" + uuid.NewString()
		}
	}
}

func ensureToolCallID(call *llm.ToolCall) {
	if call == nil || call.ID != "" {
		return
	}
	call.ID = "call_" + uuid.NewString()
}
