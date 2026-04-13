package runtime_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thkx/agent/checkpoint"
	"github.com/thkx/agent/graph"
	"github.com/thkx/agent/model"
	"github.com/thkx/agent/queue"
	"github.com/thkx/agent/runtime"
	"github.com/thkx/agent/scheduler"
	"github.com/thkx/agent/tracer"
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
		NodeName:    "end",
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
	if len(got) != 2 {
		t.Fatalf("expected 2 messages after resume, got %d", len(got))
	}
	if got[0].Content != "restored" || got[1].Content != "end" {
		t.Fatalf("unexpected resumed message sequence: %#v", got)
	}
}

func TestEngineRunIgnoresOtherExecutionResults(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	taskQ := queue.NewTaskQueue(8)
	resultQ := queue.NewResultQueue(8)
	graphStore := runtime.NewMemoryGraphStore()

	engine := runtime.New(
		runtime.WithScheduler(scheduler.New(taskQ)),
		runtime.WithResultQueue(resultQ),
		runtime.WithGraphStore(graphStore),
	)

	g := graph.New("start", "end")
	g.AddNode(&testNode{name: "start"})
	g.AddNode(&testNode{name: "end"})
	g.AddEdge("start", "end")

	if err := resultQ.PushResult(ctx, &model.TaskResult{
		ExecutionID: "other-exec",
		NodeName:    "end",
		State:       &model.State{},
		Error:       errors.New("other execution failed"),
	}); err != nil {
		t.Fatalf("push foreign result: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- engine.Run(ctx, "target-exec", g, &model.State{})
	}()

	task, err := taskQ.PopTask(ctx)
	if err != nil {
		t.Fatalf("pop submitted task: %v", err)
	}
	if task.ExecutionID != "target-exec" || task.NodeName != "start" {
		t.Fatalf("unexpected submitted task: %#v", task)
	}

	if err := resultQ.PushResult(ctx, &model.TaskResult{
		ExecutionID: "target-exec",
		NodeName:    "start",
		State:       &model.State{},
	}); err != nil {
		t.Fatalf("push target start result: %v", err)
	}

	task, err = taskQ.PopTask(ctx)
	if err != nil {
		t.Fatalf("pop second submitted task: %v", err)
	}
	if task.ExecutionID != "target-exec" || task.NodeName != "end" {
		t.Fatalf("unexpected follow-up task: %#v", task)
	}

	if err := resultQ.PushResult(ctx, &model.TaskResult{
		ExecutionID: "target-exec",
		NodeName:    "end",
		State:       &model.State{},
	}); err != nil {
		t.Fatalf("push target end result: %v", err)
	}

	if err := <-done; err != nil {
		t.Fatalf("expected target execution to ignore foreign result, got %v", err)
	}
}

func TestEngineAndWorkerEmitTracingSpans(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	taskQ := queue.NewTaskQueue(8)
	resultQ := queue.NewResultQueue(8)
	graphStore := runtime.NewMemoryGraphStore()
	memTracer := tracer.NewMemory()

	engine := runtime.New(
		runtime.WithScheduler(scheduler.New(taskQ)),
		runtime.WithResultQueue(resultQ),
		runtime.WithGraphStore(graphStore),
		runtime.WithTracer(memTracer),
	)

	w := worker.New(
		worker.WithTaskQueue(taskQ),
		worker.WithResultQueue(resultQ),
		worker.WithGraphStore(graphStore),
		worker.WithTracer(memTracer),
	)
	go w.Start(ctx)

	g := graph.New("start", "end")
	g.AddNode(&testNode{name: "start"})
	g.AddNode(&testNode{name: "end"})
	g.AddEdge("start", "end")

	if err := engine.Run(ctx, "trace-exec", g, &model.State{}); err != nil {
		t.Fatalf("engine run failed: %v", err)
	}

	spans := memTracer.Spans()
	if len(spans) == 0 {
		t.Fatal("expected tracing spans")
	}

	var sawRun bool
	var sawSubmit bool
	var sawWorker bool
	for _, span := range spans {
		switch span.Name {
		case "engine.run":
			sawRun = true
		case "engine.submit":
			sawSubmit = true
		case "worker.process_task":
			sawWorker = true
		}
	}
	if !sawRun || !sawSubmit || !sawWorker {
		t.Fatalf("expected run/submit/worker spans, got %#v", spans)
	}
}
