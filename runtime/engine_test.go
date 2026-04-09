package runtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/thkx/agent/checkpoint"
	"github.com/thkx/agent/graph"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/runtime"
	"github.com/thkx/agent/scheduler"
	"github.com/thkx/agent/worker"
)

type testNode struct {
	name string
	run  func(*model.State)
}

func (n *testNode) Name() string { return n.name }

func (n *testNode) Execute(ctx context.Context, state *model.State) (*model.State, error) {
	if n.run != nil {
		n.run(state)
	}
	return state, nil
}

func TestEngineRunExecutesGraphAndSavesCheckpoint(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	taskQ := queue.NewTaskQueue(8)
	resultQ := queue.NewResultQueue(8)
	graphStore := runtime.NewMemoryGraphStore()
	cp := checkpoint.New()

	engine := runtime.New(
		runtime.WithScheduler(scheduler.New(taskQ)),
		runtime.WithResultQueue(resultQ),
		runtime.WithGraphStore(graphStore),
		runtime.WithCheckpoint(cp),
	)

	w := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphStore(graphStore),
	)
	go w.Start(ctx)

	g := graph.New("start", "end")
	g.AddNode(&testNode{
		name: "start",
		run: func(state *model.State) {
			state.Messages = append(state.Messages, model.Message{Role: "system", Content: "start"})
		},
	})
	g.AddNode(&testNode{
		name: "end",
		run: func(state *model.State) {
			state.Messages = append(state.Messages, model.Message{Role: "system", Content: "end"})
		},
	})
	g.AddEdge("start", "end")

	state := &model.State{}
	if err := engine.Run(ctx, "exec-run", g, state); err != nil {
		t.Fatalf("engine run failed: %v", err)
	}

	if len(state.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(state.Messages))
	}
	if state.Messages[0].Content != "start" || state.Messages[1].Content != "end" {
		t.Fatalf("unexpected message sequence: %#v", state.Messages)
	}

	snapshot := cp.Load(ctx, "exec-run")
	if snapshot == nil {
		t.Fatal("expected checkpoint snapshot, got nil")
	}
	if snapshot.NodeName != "end" {
		t.Fatalf("expected checkpoint node %q, got %q", "end", snapshot.NodeName)
	}
}

func TestEngineRunResumesFromCheckpoint(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	taskQ := queue.NewTaskQueue(8)
	resultQ := queue.NewResultQueue(8)
	graphStore := runtime.NewMemoryGraphStore()
	cp := checkpoint.New()

	engine := runtime.New(
		runtime.WithScheduler(scheduler.New(taskQ)),
		runtime.WithResultQueue(resultQ),
		runtime.WithGraphStore(graphStore),
		runtime.WithCheckpoint(cp),
	)

	w := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphStore(graphStore),
	)
	go w.Start(ctx)

	g := graph.New("start", "end")
	g.AddNode(&testNode{
		name: "start",
		run: func(state *model.State) {
			state.Messages = append(state.Messages, model.Message{Role: "system", Content: "start"})
		},
	})
	g.AddNode(&testNode{
		name: "middle",
		run: func(state *model.State) {
			state.Messages = append(state.Messages, model.Message{Role: "system", Content: "middle"})
		},
	})
	g.AddNode(&testNode{
		name: "end",
		run: func(state *model.State) {
			state.Messages = append(state.Messages, model.Message{Role: "system", Content: "end"})
		},
	})
	g.AddEdge("start", "middle")
	g.AddEdge("middle", "end")

	cp.Save(ctx, &model.ExecutionSnapshot{
		ExecutionID: "exec-resume",
		NodeName:    "middle",
		State: &model.State{
			Messages: []model.Message{
				{Role: "system", Content: "restored"},
			},
		},
	})

	state := &model.State{
		Messages: []model.Message{
			{Role: "system", Content: "fresh"},
		},
	}
	if err := engine.Run(ctx, "exec-resume", g, state); err != nil {
		t.Fatalf("engine run failed: %v", err)
	}

	snapshot := cp.Load(ctx, "exec-resume")
	if snapshot == nil {
		t.Fatal("expected checkpoint snapshot, got nil")
	}

	got := snapshot.State.Messages
	if len(got) != 3 {
		t.Fatalf("expected 3 messages after resume, got %d", len(got))
	}
	if got[0].Content != "restored" || got[1].Content != "middle" || got[2].Content != "end" {
		t.Fatalf("unexpected resumed message sequence: %#v", got)
	}
}
