package distributed

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/thkx/agent/model"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/runtime"
	"github.com/thkx/agent/worker"
)

// DistributedWorker 分布式 worker 节点
type DistributedWorker struct {
	nodeID         string
	taskQueue      *queue.TaskQueue
	resultQueue    *queue.ResultQueue
	graphStore     runtime.GraphStore
	localWorker    *worker.Worker
	transport      Transport
	mu             sync.RWMutex
	running        bool
	pendingResults map[string]*model.TaskResult
	resultChan     chan *model.TaskResult
}

// Transport 分布式传输接口
type Transport interface {
	SendTaskResult(ctx context.Context, result *model.TaskResult) error
	SendTask(task *model.Task) error
	ReceiveTask(ctx context.Context) (*model.Task, error)
	ReceiveResult(ctx context.Context) (*model.TaskResult, error)
	Close() error
}

// NewDistributedWorker 创建分布式 worker
func NewDistributedWorker(
	nodeID string,
	taskQ *queue.TaskQueue,
	resultQ *queue.ResultQueue,
	graphStore runtime.GraphStore,
	transport Transport,
) *DistributedWorker {
	// 创建本地 worker
	localWorker := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphStore(graphStore),
	)

	return &DistributedWorker{
		nodeID:         nodeID,
		taskQueue:      taskQ,
		resultQueue:    resultQ,
		graphStore:     graphStore,
		localWorker:    localWorker,
		transport:      transport,
		pendingResults: make(map[string]*model.TaskResult),
		resultChan:     make(chan *model.TaskResult, 100),
	}
}

// Start 启动分布式 worker
func (dw *DistributedWorker) Start(ctx context.Context) {
	dw.mu.Lock()
	if dw.running {
		dw.mu.Unlock()
		return
	}
	dw.running = true
	dw.mu.Unlock()

	log.Printf("[%s] Distributed worker started", dw.nodeID)

	// 启动本地 worker
	go dw.localWorker.Start(ctx)

	// 启动结果转发
	go dw.forwardResults(ctx)

	// 启动远程任务接收（如果需要）
	go dw.receiveRemoteTasks(ctx)
}

// Stop 停止分布式 worker
func (dw *DistributedWorker) Stop() error {
	dw.mu.Lock()
	dw.running = false
	dw.mu.Unlock()

	close(dw.resultChan)
	return dw.transport.Close()
}

// forwardResults 将结果转发到远程
func (dw *DistributedWorker) forwardResults(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case result, ok := <-dw.resultChan:
			if !ok {
				return
			}

			// 通过传输层发送结果
			if err := dw.transport.SendTaskResult(ctx, result); err != nil {
				log.Printf("[%s] Failed to send result: %v (ExecutionID: %s)", dw.nodeID, err, result.ExecutionID)
			} else {
				log.Printf("[%s] Sent result for task %s", dw.nodeID, result.ExecutionID)
			}
		}
	}
}

// receiveRemoteTasks 接收远程任务（可选）
func (dw *DistributedWorker) receiveRemoteTasks(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			task, err := dw.transport.ReceiveTask(ctx)
			if err != nil {
				continue
			}

			// 将远程任务推送到本地队列
			if task != nil {
				_ = dw.taskQueue.PushTask(ctx, task)
				log.Printf("[%s] Received remote task: %s", dw.nodeID, task.ExecutionID)
			}
		}
	}
}

// SubmitResult 提交任务结果到结果队列
func (dw *DistributedWorker) SubmitResult(ctx context.Context, result *model.TaskResult) error {
	dw.mu.RLock()
	defer dw.mu.RUnlock()

	if !dw.running {
		return fmt.Errorf("worker is not running")
	}

	select {
	case dw.resultChan <- result:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetNodeID 获取节点 ID
func (dw *DistributedWorker) GetNodeID() string {
	return dw.nodeID
}

// GetStats 获取分布式 worker 统计信息
type WorkerStats struct {
	NodeID         string `json:"node_id"`
	PendingResults int    `json:"pending_results"`
	Running        bool   `json:"running"`
}

func (dw *DistributedWorker) GetStats() WorkerStats {
	dw.mu.RLock()
	defer dw.mu.RUnlock()

	return WorkerStats{
		NodeID:         dw.nodeID,
		PendingResults: len(dw.pendingResults),
		Running:        dw.running,
	}
}
