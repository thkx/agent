package graph

import (
	"fmt"

	"github.com/thkx/agent/model"
)

type Edge struct {
	To        string
	Condition func(*model.State) (string, bool)
}

type Graph struct {
	nodes map[string]Node
	edges map[string][]Edge
	start string
	end   string
}

func New(start, end string) *Graph {
	return &Graph{
		nodes: make(map[string]Node),
		edges: make(map[string][]Edge),
		start: start,
		end:   end,
	}
}

func (g *Graph) AddNode(n Node) {
	g.nodes[n.Name()] = n
}

func (g *Graph) AddEdge(from, to string) {
	g.edges[from] = append(g.edges[from], Edge{To: to})
}

func (g *Graph) AddConditionalEdge(from string, fn func(*model.State) (string, bool)) {
	g.edges[from] = append(g.edges[from], Edge{Condition: fn})
}

func (g *Graph) Start() string {
	return g.start
}

func (g *Graph) End() string {
	return g.end
}

func (g *Graph) GetNode(name string) (Node, bool) {
	if g == nil {
		return nil, false
	}

	n, ok := g.nodes[name]
	return n, ok
}

func (g *Graph) Next(node string, state *model.State) (string, error) {
	for _, e := range g.edges[node] {
		if e.Condition != nil {
			if next, ok := e.Condition(state); ok {
				return next, nil
			}
		} else {
			return e.To, nil
		}
	}
	return "", fmt.Errorf("no next node from %s", node)
}

func (g *Graph) Validate() error {
	if !g.HasNode(g.start) {
		return fmt.Errorf("start node %s does not exist", g.start)
	}
	if !g.HasNode(g.end) {
		return fmt.Errorf("end node %s does not exist", g.end)
	}
	for from, edges := range g.edges {
		if !g.HasNode(from) {
			return fmt.Errorf("edge from non-existent node %s", from)
		}
		for _, e := range edges {
			if e.To != "" && !g.HasNode(e.To) {
				return fmt.Errorf("edge to non-existent node %s", e.To)
			}
		}
	}
	return nil
}

func (g *Graph) RemoveNode(name string) {
	delete(g.nodes, name)
	delete(g.edges, name)
	// 清理指向该节点的边
	for from, edges := range g.edges {
		var filtered []Edge
		for _, e := range edges {
			if e.To != name {
				filtered = append(filtered, e)
			}
		}
		g.edges[from] = filtered
	}
}

func (g *Graph) AddDynamicNode(n Node) {
	g.AddNode(n)
}

func (g *Graph) HasNode(name string) bool {
	_, ok := g.nodes[name]
	return ok
}

func (g *Graph) Clone() *Graph {
	if g == nil {
		return nil
	}

	cloned := &Graph{
		nodes: make(map[string]Node, len(g.nodes)),
		edges: make(map[string][]Edge, len(g.edges)),
		start: g.start,
		end:   g.end,
	}

	for name, node := range g.nodes {
		cloned.nodes[name] = node
	}

	for from, edges := range g.edges {
		cloned.edges[from] = append([]Edge(nil), edges...)
	}

	return cloned
}
