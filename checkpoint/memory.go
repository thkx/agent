package checkpoint

import (
	"context"
	"sync"

	"github.com/thkx/agent/model"
)

type Store struct {
	data map[string]*model.ExecutionSnapshot
	mu   sync.RWMutex
}

func New() *Store {
	return &Store{
		data: make(map[string]*model.ExecutionSnapshot),
	}
}

func (s *Store) Save(ctx context.Context, snapshot *model.ExecutionSnapshot) {
	if snapshot == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[snapshot.ExecutionID] = snapshot.Clone()
}

func (s *Store) Load(ctx context.Context, execID string) *model.ExecutionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := s.data[execID]
	return snapshot.Clone()
}
