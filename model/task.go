package model

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
