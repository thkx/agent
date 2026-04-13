package queue

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/thkx/agent/model"
)

// PersistentQueuer 持久化队列接口
type PersistentQueuer interface {
	Queuer
	Persist() error // 持久化到存储
	Recover() error // 从存储恢复
}

// HybridQueue 混合队列：内存 + 持久化
type HybridQueue struct {
	memory        *MemoryQueue
	persistence   PersistentQueuer
	enablePersist bool
}

// NewHybridQueue 创建混合队列（内存 + 持久化）
func NewHybridQueue(size int, persistence PersistentQueuer) *HybridQueue {
	return &HybridQueue{
		memory:        NewMemoryQueue(size),
		persistence:   persistence,
		enablePersist: persistence != nil,
	}
}

// Push 推送数据到队列
func (q *HybridQueue) Push(ctx context.Context, item any) error {
	// 先推送到内存队列
	if err := q.memory.Push(ctx, item); err != nil {
		return err
	}

	// 如果启用持久化，也推送到持久化层
	if q.enablePersist && q.persistence != nil {
		_ = q.persistence.Push(ctx, item)
	}

	return nil
}

// Pop 从队列弹出数据
func (q *HybridQueue) Pop(ctx context.Context) (any, error) {
	return q.memory.Pop(ctx)
}

// Persist 持久化队列状态
func (q *HybridQueue) Persist() error {
	if !q.enablePersist || q.persistence == nil {
		return nil
	}
	return q.persistence.Persist()
}

// Recover 从持久化存储恢复
func (q *HybridQueue) Recover() error {
	if !q.enablePersist || q.persistence == nil {
		return nil
	}
	return q.persistence.Recover()
}

// SerializedTask 任务序列化表示
type SerializedTask struct {
	ExecutionID string          `json:"execution_id"`
	NodeName    string          `json:"node_name"`
	State       json.RawMessage `json:"state"`
	Retry       int             `json:"retry"`
}

// SerializeTask 序列化任务
func SerializeTask(t *model.Task) ([]byte, error) {
	stateData, err := json.Marshal(t.State)
	if err != nil {
		return nil, err
	}

	st := SerializedTask{
		ExecutionID: t.ExecutionID,
		NodeName:    t.NodeName,
		State:       stateData,
		Retry:       t.Retry,
	}

	return json.Marshal(st)
}

// DeserializeTask 反序列化任务
func DeserializeTask(data []byte) (*model.Task, error) {
	var st SerializedTask
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}

	var state *model.State
	if err := json.Unmarshal(st.State, &state); err != nil {
		return nil, err
	}

	return &model.Task{
		ExecutionID: st.ExecutionID,
		NodeName:    st.NodeName,
		State:       state,
		Retry:       st.Retry,
	}, nil
}

// SerializedResult 结果序列化表示
type SerializedResult struct {
	ExecutionID string          `json:"execution_id"`
	NodeName    string          `json:"node_name"`
	State       json.RawMessage `json:"state"`
	Error       string          `json:"error"`
}

// SerializeResult 序列化结果
func SerializeResult(r *model.TaskResult) ([]byte, error) {
	stateData, err := json.Marshal(r.State)
	if err != nil {
		return nil, err
	}

	errStr := ""
	if r.Error != nil {
		errStr = r.Error.Error()
	}

	sr := SerializedResult{
		ExecutionID: r.ExecutionID,
		NodeName:    r.NodeName,
		State:       stateData,
		Error:       errStr,
	}

	return json.Marshal(sr)
}

// DeserializeResult 反序列化结果
func DeserializeResult(data []byte) (*model.TaskResult, error) {
	var sr SerializedResult
	if err := json.Unmarshal(data, &sr); err != nil {
		return nil, err
	}

	var state *model.State
	if err := json.Unmarshal(sr.State, &state); err != nil {
		return nil, err
	}

	var resultErr error
	if sr.Error != "" {
		resultErr = errors.New(sr.Error)
	}

	return &model.TaskResult{
		ExecutionID: sr.ExecutionID,
		NodeName:    sr.NodeName,
		State:       state,
		Error:       resultErr,
	}, nil
}
