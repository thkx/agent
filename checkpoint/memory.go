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

	s.data[snapshot.ExecutionID] = snapshot
}

func (s *Store) Load(ctx context.Context, execID string) *model.ExecutionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[execID]
}
