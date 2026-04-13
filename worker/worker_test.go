package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thkx/agent/graph"
	"github.com/thkx/agent/model"
	agentruntime "github.com/thkx/agent/runtime"
	"github.com/thkx/agent/queue"
)

type workerTestNode struct {
	name string
	run  func(*model.State) (*model.State, error)
}

func (n *workerTestNode) Name() string { return n.name }

func (n *workerTestNode) Execute(ctx context.Context, state *model.State) (*model.State, error) {
	if n.run != nil {
		return n.run(state)
	}
	return state, nil
}

func TestWorkerRetryKeepsOriginalTaskStateClean(t *testing.T) {
	t.Parallel()

	taskQ := queue.NewTaskQueue(8)
	resultQ := queue.NewResultQueue(8)
	graphStore := agentruntime.NewMemoryGraphStore()

	g := graph.New("node", "node")
	g.AddNode(&workerTestNode{
		name: "node",
		run: func(state *model.State) (*model.State, error) {
			state.Messages = append(state.Messages, model.Message{Role: "system", Content: "mutated"})
			return state, errors.New("boom")
		},
	})
	graphStore.Save("exec-retry", g)

	w := New(
		WithTaskQueue(taskQ),
		WithResultQueue(resultQ),
		WithGraphStore(graphStore),
		WithConcurrency(1),
	)

	task := &model.Task{
		ExecutionID: "exec-retry",
		NodeName:    "node",
		State: &model.State{
			Messages: []model.Message{{Role: "user", Content: "original"}},
		},
	}

	w.processTask(context.Background(), task)

	if len(task.State.Messages) != 1 {
		t.Fatalf("expected original task state to stay clean, got %#v", task.State.Messages)
	}

	popCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	retried, err := taskQ.PopTask(popCtx)
	if err != nil {
		t.Fatalf("pop retried task: %v", err)
	}
	if retried.Retry != 1 {
		t.Fatalf("expected retry count 1, got %d", retried.Retry)
	}
	if len(retried.State.Messages) != 1 || retried.State.Messages[0].Content != "original" {
		t.Fatalf("expected retried state to remain original, got %#v", retried.State.Messages)
	}
}

func TestWorkerStartHonorsConcurrencyLimit(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	taskQ := queue.NewTaskQueue(16)
	resultQ := queue.NewResultQueue(16)
	graphStore := agentruntime.NewMemoryGraphStore()

	var active int32
	var maxActive int32
	var started sync.WaitGroup
	started.Add(4)
	release := make(chan struct{})

	g := graph.New("node", "node")
	g.AddNode(&workerTestNode{
		name: "node",
		run: func(state *model.State) (*model.State, error) {
			current := atomic.AddInt32(&active, 1)
			for {
				prev := atomic.LoadInt32(&maxActive)
				if current <= prev || atomic.CompareAndSwapInt32(&maxActive, prev, current) {
					break
				}
			}
			started.Done()
			<-release
			atomic.AddInt32(&active, -1)
			return state, nil
		},
	})
	graphStore.Save("exec-concurrency", g)

	w := New(
		WithTaskQueue(taskQ),
		WithResultQueue(resultQ),
		WithGraphStore(graphStore),
		WithConcurrency(2),
	)

	go w.Start(ctx)

	for i := 0; i < 4; i++ {
		if err := taskQ.PushTask(ctx, &model.Task{
			ExecutionID: "exec-concurrency",
			NodeName:    "node",
			State:       &model.State{},
		}); err != nil {
			t.Fatalf("push task %d: %v", i, err)
		}
	}

	waitDone := make(chan struct{})
	go func() {
		started.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		t.Fatal("expected concurrency limit to prevent all tasks from starting at once")
	case <-time.After(150 * time.Millisecond):
	}

	close(release)

	for i := 0; i < 4; i++ {
		if _, err := resultQ.PopResult(ctx); err != nil {
			t.Fatalf("pop result %d: %v", i, err)
		}
	}

	if got := atomic.LoadInt32(&maxActive); got > 2 {
		t.Fatalf("expected max active workers <= 2, got %d", got)
	}
}
