package taskqueue

import (
	"context"
	"encoding/json"

	"enterprise-rag/backend/internal/infrastructure/observability"
	"enterprise-rag/backend/internal/task"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

func Publish(ctx context.Context, connection *nats.Conn, subject, taskID, docID, processingMode string) error {
	payload, err := json.Marshal(task.Message{
		TaskID:         taskID,
		DocID:          docID,
		ProcessingMode: task.NormalizeProcessingMode(processingMode),
	})
	if err != nil {
		return err
	}
	ctx, span := observability.StartSpan(ctx, "nats.publish",
		attribute.String("messaging.system", "nats"),
		attribute.String("messaging.destination", subject),
	)
	message := nats.NewMsg(subject)
	message.Data = payload
	injectContext(ctx, message)
	err = connection.PublishMsg(message)
	observability.EndSpan(span, err)
	return err
}

func injectContext(ctx context.Context, message *nats.Msg) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(message.Header))
}

func ExtractContext(ctx context.Context, message *nats.Msg) context.Context {
	if message == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(message.Header))
}
