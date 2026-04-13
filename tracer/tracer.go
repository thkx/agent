package tracer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type Span struct {
	Name        string
	ExecutionID string
	NodeName    string
	PluginName  string
	Input       any
	Output      any
	Error       string
	StartedAt   time.Time
	EndedAt     time.Time
}

type Tracer interface {
	StartSpan(context.Context, Span) (context.Context, func(output any, err error))
}

type noopTracer struct{}

func NewNoop() Tracer {
	return noopTracer{}
}

func (noopTracer) StartSpan(ctx context.Context, span Span) (context.Context, func(output any, err error)) {
	return ctx, func(output any, err error) {}
}

type MemoryTracer struct {
	mu    sync.Mutex
	spans []Span
}

func NewMemory() *MemoryTracer {
	return &MemoryTracer{}
}

func (t *MemoryTracer) StartSpan(ctx context.Context, span Span) (context.Context, func(output any, err error)) {
	span.StartedAt = time.Now()
	return ctx, func(output any, err error) {
		span.EndedAt = time.Now()
		span.Output = output
		if err != nil {
			span.Error = err.Error()
		}

		t.mu.Lock()
		defer t.mu.Unlock()
		t.spans = append(t.spans, span)
	}
}

func (t *MemoryTracer) Spans() []Span {
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make([]Span, len(t.spans))
	copy(out, t.spans)
	return out
}

func (t *MemoryTracer) JSON() ([]byte, error) {
	return json.MarshalIndent(t.Spans(), "", "  ")
}

type OTELTracer struct {
	tracer oteltrace.Tracer
}

func NewOTEL(serviceName string) (Tracer, error) {
	// Initialize OTEL tracer provider with stdout exporter for simplicity
	tp, err := initTracerProvider(serviceName)
	if err != nil {
		return nil, err
	}
	otel.SetTracerProvider(tp)
	tracer := otel.Tracer("agent-runtime")
	return &OTELTracer{tracer: tracer}, nil
}

func (t *OTELTracer) StartSpan(ctx context.Context, span Span) (context.Context, func(output any, err error)) {
	ctx, otelSpan := t.tracer.Start(ctx, span.Name,
		oteltrace.WithAttributes(
			attribute.String("execution_id", span.ExecutionID),
			attribute.String("node_name", span.NodeName),
			attribute.String("plugin_name", span.PluginName),
		),
	)
	return ctx, func(output any, err error) {
		if err != nil {
			otelSpan.RecordError(err)
			otelSpan.SetStatus(codes.Error, err.Error())
		}
		otelSpan.End()
	}
}

func initTracerProvider(serviceName string) (*trace.TracerProvider, error) {
	// Simple stdout exporter for demo
	exporter, err := NewStdoutExporter()
	if err != nil {
		return nil, err
	}
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
	)
	return tp, nil
}

// Simple stdout exporter (for demo purposes)
type StdoutExporter struct{}

func NewStdoutExporter() (*StdoutExporter, error) {
	return &StdoutExporter{}, nil
}

func (e *StdoutExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	for _, span := range spans {
		data := map[string]interface{}{
			"name":       span.Name(),
			"trace_id":   span.SpanContext().TraceID().String(),
			"span_id":    span.SpanContext().SpanID().String(),
			"start_time": span.StartTime(),
			"end_time":   span.EndTime(),
			"attributes": span.Attributes(),
			"status":     span.Status(),
		}
		if span.Status().Code == codes.Error {
			data["error"] = span.Status().Description
		}
		jsonData, _ := json.Marshal(data)
		fmt.Println(string(jsonData))
	}
	return nil
}

func (e *StdoutExporter) Shutdown(ctx context.Context) error {
	return nil
}
