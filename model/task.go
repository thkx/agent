package model

import (
	"time"
)

const MAX_RETRY = 3

type Task struct {
	ExecutionID string
	NodeName    string
	State       *State
	Retry       int // 已重试次数
}

type ExecutionSnapshot struct {
	ExecutionID string
	NodeName    string
	State       *State
}

type TaskResult struct {
	ExecutionID string
	NodeName    string
	State       *State
	Error       error
}

// RetryableError 表示可重试的错误
type RetryableError struct {
	Err        error
	RetryAfter time.Duration
}

func (e RetryableError) Error() string {
	return e.Err.Error()
}

func (e RetryableError) Unwrap() error {
	return e.Err
}

func (e RetryableError) ShouldRetry() bool {
	return true
}

// FatalError 表示致命错误，不应重试
type FatalError struct {
	Err error
}

func (e FatalError) Error() string {
	return e.Err.Error()
}

func (e FatalError) Unwrap() error {
	return e.Err
}

func (e FatalError) ShouldRetry() bool {
	return false
}

func (t *Task) Clone() *Task {
	if t == nil {
		return nil
	}

	return &Task{
		ExecutionID: t.ExecutionID,
		NodeName:    t.NodeName,
		State:       t.State.Clone(),
		Retry:       t.Retry,
	}
}

func (s *ExecutionSnapshot) Clone() *ExecutionSnapshot {
	if s == nil {
		return nil
	}

	return &ExecutionSnapshot{
		ExecutionID: s.ExecutionID,
		NodeName:    s.NodeName,
		State:       s.State.Clone(),
	}
}

func (r *TaskResult) Clone() *TaskResult {
	if r == nil {
		return nil
	}

	return &TaskResult{
		ExecutionID: r.ExecutionID,
		NodeName:    r.NodeName,
		State:       r.State.Clone(),
		Error:       r.Error,
	}
}
