package tracer

import (
	"context"
	"strings"
	"testing"
)

func TestMemoryTracerJSON(t *testing.T) {
	t.Parallel()

	mem := NewMemory()
	_, finish := mem.StartSpan(context.Background(), Span{
		Name:        "engine.run",
		ExecutionID: "exec-1",
	})
	finish(map[string]any{"ok": true}, nil)

	data, err := mem.JSON()
	if err != nil {
		t.Fatalf("json export failed: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"engine.run"`) || !strings.Contains(text, `"exec-1"`) {
		t.Fatalf("unexpected json export: %s", text)
	}
}
