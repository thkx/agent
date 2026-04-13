package toolruntime

import (
	"context"
	"testing"
	"time"

	"github.com/thkx/agent/tool"
)

type stubTool struct {
	name   string
	run    func(input any) (any, error)
	schema tool.Schema
}

func (t *stubTool) Name() string { return t.name }

func (t *stubTool) Description() string { return "stub tool" }

func (t *stubTool) Schema() tool.Schema {
	if t.schema.Type != "" {
		return t.schema
	}
	return tool.Schema{
		Type: "object",
	}
}

func (t *stubTool) Invoke(ctx context.Context, input any) (any, error) {
	if t.run != nil {
		return t.run(input)
	}
	return nil, nil
}

func (t *stubTool) Timeout() time.Duration {
	return 10 * time.Second
}

func (t *stubTool) Permissions() []string {
	return []string{"read"}
}

func TestLocalRuntimeExecuteAndList(t *testing.T) {
	t.Parallel()

	registry := tool.NewRegistry(
		&stubTool{name: "b"},
		&stubTool{name: "a"},
	)
	rt := NewLocal(registry)

	defs := registry.Definitions()
	if len(defs) != 2 || defs[0].Name != "a" || defs[1].Name != "b" {
		t.Fatalf("expected sorted definitions, got %#v", defs)
	}
	if defs[0].Description == "" || defs[0].Schema.Type != "object" {
		t.Fatalf("expected definition metadata to be populated, got %#v", defs[0])
	}

	_, err := rt.Execute(context.Background(), ToolCall{Name: "missing"})
	if err == nil {
		t.Fatal("expected missing tool error")
	}
}

func TestLocalRuntimeValidatesInputAgainstSchema(t *testing.T) {
	t.Parallel()

	registry := tool.NewRegistry(
		&stubTool{
			name: "price",
			run: func(input any) (any, error) {
				return "ok", nil
			},
			schema: tool.Schema{
				Type: "object",
				Properties: map[string]tool.Property{
					"symbol": {Type: "string"},
				},
				Required: []string{"symbol"},
			},
		},
	)
	rt := NewLocal(registry)

	_, err := rt.Execute(context.Background(), ToolCall{
		Name:  "price",
		Input: map[string]any{},
	})
	if err == nil {
		t.Fatal("expected required-field validation error")
	}

	_, err = rt.Execute(context.Background(), ToolCall{
		Name: "price",
		Input: map[string]any{
			"symbol": 123,
		},
	})
	if err == nil {
		t.Fatal("expected type validation error")
	}
}
