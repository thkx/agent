package distributed

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/thkx/agent/model"
)

// Coordinator 分布式协调器，管理多个 Worker 节点
type Coordinator struct {
	nodeID    string
	nodes     map[string]*NodeInfo
	nodesMu   sync.RWMutex
	transport map[string]Transport
	taskChan  chan *model.Task
	resultChan chan *model.TaskResult
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// NodeInfo 节点信息
type NodeInfo struct {
	NodeID   string
	Status   string // active, idle, failed
	TasksProcessed int
	LastSeen int64
}

// NewCoordinator 创建协调器
func NewCoordinator(nodeID string) *Coordinator {
	return &Coordinator{
		nodeID:     nodeID,
		nodes:      make(map[string]*NodeInfo),
		transport:  make(map[string]Transport),
		taskChan:   make(chan *model.Task, 100),
		resultChan: make(chan *model.TaskResult, 100),
		stopChan:   make(chan struct{}),
	}
}

// RegisterNode 注册 worker 节点
func (c *Coordinator) RegisterNode(nodeID string, transport Transport) error {
	c.nodesMu.Lock()
	defer c.nodesMu.Unlock()

	if _, exists := c.nodes[nodeID]; exists {
		return ErrNodeAlreadyExists
	}

	c.nodes[nodeID] = &NodeInfo{
		NodeID: nodeID,
		Status: "active",
	}
	c.transport[nodeID] = transport

	log.Printf("[Coordinator] Registered node: %s", nodeID)
	return nil
}

// UnregisterNode 注销 worker 节点
func (c *Coordinator) UnregisterNode(nodeID string) error {
	c.nodesMu.Lock()
	defer c.nodesMu.Unlock()

	if _, exists := c.nodes[nodeID]; !exists {
		return ErrNodeNotFound
	}

	delete(c.nodes, nodeID)
	delete(c.transport, nodeID)

	log.Printf("[Coordinator] Unregistered node: %s", nodeID)
	return nil
}

// DistributeTask 分配任务到 worker 节点
func (c *Coordinator) DistributeTask(ctx context.Context, task *model.Task) error {
	c.nodesMu.RLock()
	nodes := make([]string, 0, len(c.nodes))
	for nodeID := range c.nodes {
		nodes = append(nodes, nodeID)
	}
	c.nodesMu.RUnlock()

	if len(nodes) == 0 {
		return fmt.Errorf("no available nodes")
	}

	// 简单的轮询分配策略
	selectedNode := nodes[0]

	c.nodesMu.RLock()
	transport, ok := c.transport[selectedNode]
	c.nodesMu.RUnlock()

	if !ok {
		return fmt.Errorf("transport for node %s not found", selectedNode)
	}

	// 将任务发送给选中的节点
	if err := transport.SendTask(task); err != nil {
		return fmt.Errorf("failed to send task to node %s: %w", selectedNode, err)
	}

	log.Printf("[Coordinator] Distributed task %s to node %s", task.ExecutionID, selectedNode)
	return nil
}

// CollectResults 收集所有节点的结果
func (c *Coordinator) CollectResults(ctx context.Context) *model.TaskResult {
	c.nodesMu.RLock()
	nodes := make([]string, 0, len(c.nodes))
	for nodeID := range c.nodes {
		nodes = append(nodes, nodeID)
	}
	c.nodesMu.RUnlock()

	for _, nodeID := range nodes {
		c.nodesMu.RLock()
		transport, ok := c.transport[nodeID]
		c.nodesMu.RUnlock()

		if !ok {
			continue
		}

		result, err := transport.ReceiveResult(ctx)
		if err != nil {
			continue
		}

		if result != nil {
			return result
		}
	}

	return nil
}

// Start 启动协调器
func (c *Coordinator) Start(ctx context.Context) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.run(ctx)
	}()
}

// run 协调器主循环
func (c *Coordinator) run(ctx context.Context) {
	ticker := make(chan struct{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopChan:
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case task := <-c.taskChan:
			_ = c.DistributeTask(ctx, task)
		case result := <-c.resultChan:
			// 处理结果
			log.Printf("[Coordinator] Received result for task %s", result.ExecutionID)
		case <-ticker:
			c.reportStats()
		}
	}
}

// Stop 停止协调器
func (c *Coordinator) Stop() {
	close(c.stopChan)
	c.wg.Wait()
}

// reportStats 报告统计信息
func (c *Coordinator) reportStats() {
	c.nodesMu.RLock()
	defer c.nodesMu.RUnlock()

	log.Printf("[Coordinator] Active nodes: %d", len(c.nodes))
	for nodeID, info := range c.nodes {
		log.Printf("  - %s: %s (tasks processed: %d)", nodeID, info.Status, info.TasksProcessed)
	}
}

// GetNodeStats 获取所有节点的统计信息
func (c *Coordinator) GetNodeStats() map[string]NodeInfo {
	c.nodesMu.RLock()
	defer c.nodesMu.RUnlock()

	stats := make(map[string]NodeInfo)
	for nodeID, info := range c.nodes {
		stats[nodeID] = *info
	}
	return stats
}
