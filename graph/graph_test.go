package graph

import (
	"context"
	"testing"

	"github.com/thkx/agent/model"
)

type testNode struct {
	name string
}

func (n *testNode) Name() string {
	return n.name
}

func (n *testNode) Execute(ctx context.Context, state *model.State) (*model.State, error) {
	return state, nil
}

func TestGraphValidate(t *testing.T) {
	t.Parallel()

	// 有效的图
	g := New("start", "end")
	g.AddNode(&testNode{name: "start"})
	g.AddNode(&testNode{name: "end"})
	g.AddEdge("start", "end")

	if err := g.Validate(); err != nil {
		t.Fatalf("expected valid graph, got error: %v", err)
	}

	// 无效的图：缺少开始节点
	g2 := New("missing", "end")
	g2.AddNode(&testNode{name: "end"})
	if err := g2.Validate(); err == nil {
		t.Fatalf("expected error for missing start node")
	}

	// 无效的图：边指向不存在的节点
	g3 := New("start", "end")
	g3.AddNode(&testNode{name: "start"})
	g3.AddNode(&testNode{name: "end"})
	g3.AddEdge("start", "missing")
	if err := g3.Validate(); err == nil {
		t.Fatalf("expected error for edge to missing node")
	}
}