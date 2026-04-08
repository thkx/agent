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
