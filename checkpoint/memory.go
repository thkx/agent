package checkpoint

import (
	"context"
	"sync"

	"github.com/thkx/agent/model"
)

type Store struct {
	data map[string]*model.Task
	mu   sync.RWMutex
}

func New() *Store {
	return &Store{
		data: make(map[string]*model.Task),
	}
}

func (s *Store) Save(ctx context.Context, execID, node string, state *model.State) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[execID] = &model.Task{
		ExecutionID: execID,
		NodeName:    node,
		State:       state,
	}
}

func (s *Store) Load(ctx context.Context, execID string) *model.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[execID]
}
