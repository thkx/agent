package runtime

import (
	"context"
	"testing"

	"github.com/thkx/agent/graph"
	"github.com/thkx/agent/model"
)

type graphStoreTestNode struct {
	name string
}

func (n *graphStoreTestNode) Name() string { return n.name }

func (n *graphStoreTestNode) Execute(_ context.Context, state *model.State) (*model.State, error) {
	return state, nil
}

func TestMemoryGraphStoreClonesOnSaveAndLoad(t *testing.T) {
	t.Parallel()

	store := NewMemoryGraphStore()

	g := graph.New("start", "end")
	g.AddNode(&graphStoreTestNode{name: "start"})
	g.AddNode(&graphStoreTestNode{name: "end"})
	g.AddEdge("start", "end")

	store.Save("exec-1", g)

	g.AddNode(&graphStoreTestNode{name: "late"})
	g.AddEdge("end", "late")

	loaded, ok := store.Load("exec-1")
	if !ok || loaded == nil {
		t.Fatal("expected stored graph")
	}
	if _, ok := loaded.GetNode("late"); ok {
		t.Fatal("expected save to isolate later graph mutations")
	}

	loaded.AddNode(&graphStoreTestNode{name: "mutated"})
	loadedAgain, ok := store.Load("exec-1")
	if !ok || loadedAgain == nil {
		t.Fatal("expected stored graph on second load")
	}
	if _, ok := loadedAgain.GetNode("mutated"); ok {
		t.Fatal("expected load to return an isolated graph clone")
	}
}
