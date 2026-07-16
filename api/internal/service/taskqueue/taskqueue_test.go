package taskqueue

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestTraceContextRoundTrip(t *testing.T) {
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})

	ctx, span := provider.Tracer("test").Start(context.Background(), "upload")
	message := nats.NewMsg("document.parse")
	injectContext(ctx, message)
	extracted := ExtractContext(context.Background(), message)
	span.End()

	want := span.SpanContext()
	got := trace.SpanContextFromContext(extracted)
	if !got.IsValid() || got.TraceID() != want.TraceID() || got.SpanID() != want.SpanID() {
		t.Fatalf("trace context mismatch: got=%s/%s want=%s/%s", got.TraceID(), got.SpanID(), want.TraceID(), want.SpanID())
	}
}

func TestNATSTracePropagation(t *testing.T) {
	natsURL := os.Getenv("NATS_TEST_URL")
	if natsURL == "" {
		t.Skip("NATS_TEST_URL is not set")
	}

	recorder := tracetest.NewSpanRecorder()
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(recorder),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})

	connection, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	jetStream, err := connection.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := EnsureStream(ctx, jetStream); err != nil {
		t.Fatal(err)
	}
	subscription, err := jetStream.SubscribeSync(
		"document.trace",
		nats.BindStream(StreamName),
		nats.ManualAck(),
		nats.DeliverNew(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer subscription.Unsubscribe()
	if err := connection.Flush(); err != nil {
		t.Fatal(err)
	}

	traceCtx, root := provider.Tracer("test").Start(context.Background(), "upload")
	if err := Publish(traceCtx, jetStream, "document.trace", "task", "doc", "standard"); err != nil {
		t.Fatal(err)
	}
	root.End()
	message, err := subscription.NextMsg(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := message.AckSync(); err != nil {
		t.Fatal(err)
	}
	remoteParent := trace.SpanContextFromContext(ExtractContext(context.Background(), message))
	if !remoteParent.IsRemote() || remoteParent.TraceID() != root.SpanContext().TraceID() {
		t.Fatalf("unexpected remote parent: %+v", remoteParent)
	}

	var publishSpanID trace.SpanID
	for _, span := range recorder.Ended() {
		if span.Name() == "nats.publish" {
			publishSpanID = span.SpanContext().SpanID()
		}
	}
	if !publishSpanID.IsValid() || remoteParent.SpanID() != publishSpanID {
		t.Fatalf("remote parent span = %s, publish span = %s", remoteParent.SpanID(), publishSpanID)
	}
}
