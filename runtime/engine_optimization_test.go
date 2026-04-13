package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/thkx/agent/model"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/scheduler"
)

func TestAwaitResultReducesLockContention(t *testing.T) {
	t.Parallel()

	// 这个测试验证awaitResult在首次检查后不再自旋
	resultQueue := queue.NewResultQueue(100)
	taskQueue := queue.NewTaskQueue(100)
	engine := New(
		WithScheduler(scheduler.New(taskQueue)),
		WithResultQueue(resultQueue),
	)

	// 推送一个结果到队列
	result := &model.TaskResult{
		ExecutionID: "exec-1",
		NodeName:    "test-node",
		State: &model.State{
			Messages: []model.Message{},
		},
	}

	err := resultQueue.PushResult(context.Background(), result)
	if err != nil {
		t.Fatalf("failed to push result: %v", err)
	}

	// 调用awaitResult应该立即返回，不自旋
	start := time.Now()
	res, err := engine.awaitResult(context.Background(), "exec-1")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("awaitResult failed: %v", err)
	}

	if res.ExecutionID != "exec-1" {
		t.Fatalf("expected execution ID exec-1, got %s", res.ExecutionID)
	}

	// 应该快速返回(< 100ms)，说明没有自旋
	if elapsed > 100*time.Millisecond {
		t.Fatalf("awaitResult took too long (%v), indicates possible spinning", elapsed)
	}
}

func TestAwaitResultHandlesMixedExecutionIDs(t *testing.T) {
	t.Parallel()

	resultQueue := queue.NewResultQueue(100)
	taskQueue := queue.NewTaskQueue(100)
	engine := New(
		WithScheduler(scheduler.New(taskQueue)),
		WithResultQueue(resultQueue),
	)

	// 推送混合的execution IDs:结果顺序为3,0,1,2,4
	// 当我们要求exec-2时，应该先获取3和0和1在待处理中，然后找到2
	results := []string{"exec-3", "exec-0", "exec-1", "exec-2", "exec-4"}
	for _, execID := range results {
		result := &model.TaskResult{
			ExecutionID: execID,
			NodeName:    "test-node",
			State: &model.State{
				Messages: []model.Message{},
			},
		}
		err := resultQueue.PushResult(context.Background(), result)
		if err != nil {
			t.Fatalf("failed to push result: %v", err)
		}
	}

	// 等待exec-2的结果
	res, err := engine.awaitResult(context.Background(), "exec-2")
	if err != nil {
		t.Fatalf("awaitResult failed: %v", err)
	}

	if res.ExecutionID != "exec-2" {
		t.Fatalf("expected execution ID exec-2, got %s", res.ExecutionID)
	}

	// 验证其他结果仍然在pending中
	engine.resultsMu.Lock()
	totalPending := 0
	for _, results := range engine.pending {
		totalPending += len(results)
	}
	engine.resultsMu.Unlock()

	// 应该有3个结果在pending中(exec-0, exec-1, exec-3)
	// exec-4可能还在队列中或已被跳过
	if totalPending < 3 {
		t.Fatalf("expected at least 3 results in pending, got %d", totalPending)
	}
}

func TestResultQueueChanelReplacesPolling(t *testing.T) {
	t.Parallel()

	// 验证PopResult使用channel，不会阻塞或超时
	resultQueue := queue.NewResultQueue(100)

	// 在goroutine中推送结果
	go func() {
		time.Sleep(50 * time.Millisecond)
		result := &model.TaskResult{
			ExecutionID: "exec-1",
			NodeName:    "test-node",
			State:       &model.State{Messages: []model.Message{}},
		}
		_ = resultQueue.PushResult(context.Background(), result)
	}()

	// 应该阻塞直到结果可用，而不是自旋
	start := time.Now()
	res, err := resultQueue.PopResult(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("PopResult failed: %v", err)
	}

	if res.ExecutionID != "exec-1" {
		t.Fatalf("expected exec-1, got %s", res.ExecutionID)
	}

	// 应该大约等待50ms，说明是阻塞等待，不是自旋
	if elapsed < 40*time.Millisecond || elapsed > 150*time.Millisecond {
		t.Fatalf("PopResult timing suggests polling: %v", elapsed)
	}
}
