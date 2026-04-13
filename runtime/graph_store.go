package runtime

import (
	"sync"

	"github.com/thkx/agent/graph"
)

type GraphStore interface {
	Save(execID string, g *graph.Graph)
	Load(execID string) (*graph.Graph, bool)
	Delete(execID string)
}

type MemoryGraphStore struct {
	mu    sync.RWMutex
	graph map[string]*graph.Graph
}

func NewMemoryGraphStore() *MemoryGraphStore {
	return &MemoryGraphStore{
		graph: make(map[string]*graph.Graph),
	}
}

func (s *MemoryGraphStore) Save(execID string, g *graph.Graph) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.graph[execID] = g.Clone()
}

func (s *MemoryGraphStore) Load(execID string) (*graph.Graph, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	g, ok := s.graph[execID]
	return g.Clone(), ok
}

func (s *MemoryGraphStore) Delete(execID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.graph, execID)
}
