package observability

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSpanLifecycle(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})

	ctx, parent := StartSpan(context.Background(), "parent")
	_, child := StartSpan(ctx, "child")
	EndSpan(child, errors.New("failed"))
	EndSpanWithStatus(parent, "success")

	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("expected 2 ended spans, got %d", len(spans))
	}
	if spans[0].Parent().SpanID() != spans[1].SpanContext().SpanID() {
		t.Fatal("child span does not reference the parent span")
	}
}
