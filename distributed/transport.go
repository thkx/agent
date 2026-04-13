package distributed

import (
	"context"
	"sync"

	"github.com/thkx/agent/model"
)

// InMemoryTransport 内存传输实现（用于本地测试）
type InMemoryTransport struct {
	taskChan   chan *model.Task
	resultChan chan *model.TaskResult
	mu         sync.RWMutex
	closed     bool
}

// NewInMemoryTransport 创建内存传输
func NewInMemoryTransport() *InMemoryTransport {
	return &InMemoryTransport{
		taskChan:   make(chan *model.Task, 100),
		resultChan: make(chan *model.TaskResult, 100),
	}
}

// SendTaskResult 发送任务结果
func (t *InMemoryTransport) SendTaskResult(ctx context.Context, result *model.TaskResult) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed {
		return ErrTransportClosed
	}

	select {
	case t.resultChan <- result:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ReceiveTask 接收任务
func (t *InMemoryTransport) ReceiveTask(ctx context.Context) (*model.Task, error) {
	t.mu.RLock()
	closed := t.closed
	t.mu.RUnlock()

	if closed {
		return nil, ErrTransportClosed
	}

	select {
	case task := <-t.taskChan:
		return task, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SendTask 发送任务（用于协调节点）
func (t *InMemoryTransport) SendTask(task *model.Task) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed {
		return ErrTransportClosed
	}

	select {
	case t.taskChan <- task:
		return nil
	default:
		return ErrChannelFull
	}
}

// ReceiveResult 接收结果（用于协调节点）
func (t *InMemoryTransport) ReceiveResult(ctx context.Context) (*model.TaskResult, error) {
	t.mu.RLock()
	closed := t.closed
	t.mu.RUnlock()

	if closed {
		return nil, ErrTransportClosed
	}

	select {
	case result := <-t.resultChan:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close 关闭传输
func (t *InMemoryTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return ErrTransportClosed
	}

	t.closed = true
	close(t.taskChan)
	close(t.resultChan)
	return nil
}

// GetChannels 获取原始通道（用于直接访问）
func (t *InMemoryTransport) GetChannels() (chan *model.Task, chan *model.TaskResult) {
	return t.taskChan, t.resultChan
}
