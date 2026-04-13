package checkpoint

import (
	"context"
	"testing"

	"github.com/thkx/agent/model"
)

func TestStoreSaveAndLoadSnapshot(t *testing.T) {
	t.Parallel()

	store := New()
	snapshot := &model.ExecutionSnapshot{
		ExecutionID: "exec-1",
		NodeName:    "tool",
		State: &model.State{
			Messages: []model.Message{
				{Role: "assistant", Content: "running"},
			},
		},
	}

	store.Save(context.Background(), snapshot)

	got := store.Load(context.Background(), "exec-1")
	if got == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if got.ExecutionID != snapshot.ExecutionID {
		t.Fatalf("expected execution id %q, got %q", snapshot.ExecutionID, got.ExecutionID)
	}
	if got.NodeName != snapshot.NodeName {
		t.Fatalf("expected node name %q, got %q", snapshot.NodeName, got.NodeName)
	}
	if got.State == snapshot.State {
		t.Fatal("expected checkpoint load to return a cloned state")
	}
	if len(got.State.Messages) != 1 || got.State.Messages[0].Content != "running" {
		t.Fatalf("unexpected cloned state contents: %#v", got.State.Messages)
	}
}
