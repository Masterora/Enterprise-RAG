package taskqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"enterprise-rag/api/internal/infrastructure/observability"
	"enterprise-rag/api/internal/task"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

const StreamName = "DOCUMENT_TASKS"
const DeadLetterStreamName = "DOCUMENT_TASKS_DLQ"
const DeadLetterSubject = "document.dlq"

var ErrPublish = errors.New("publish durable task")

func EnsureStream(ctx context.Context, jetStream nats.JetStreamContext) error {
	desired := &nats.StreamConfig{
		Name:        StreamName,
		Description: "Durable Enterprise-RAG document processing tasks",
		Subjects: []string{
			"document.parse", "document.chunk", "document.embedding", "document.delete",
		},
		Retention:         nats.WorkQueuePolicy,
		MaxConsumers:      -1,
		MaxMsgs:           -1,
		MaxBytes:          10 << 30,
		Discard:           nats.DiscardOld,
		MaxAge:            7 * 24 * time.Hour,
		MaxMsgsPerSubject: -1,
		MaxMsgSize:        1 << 20,
		Storage:           nats.FileStorage,
		Replicas:          1,
		Duplicates:        10 * time.Minute,
	}
	_, err := jetStream.StreamInfo(StreamName, nats.Context(ctx))
	if errors.Is(err, nats.ErrStreamNotFound) {
		_, err = jetStream.AddStream(desired, nats.Context(ctx))
		if err != nil {
			return fmt.Errorf("create NATS JetStream task stream: %w", err)
		}
		if err := ensureDeadLetterStream(ctx, jetStream); err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect NATS JetStream task stream: %w", err)
	}
	if _, err := jetStream.UpdateStream(desired, nats.Context(ctx)); err != nil {
		return fmt.Errorf("update NATS JetStream task stream: %w", err)
	}
	return ensureDeadLetterStream(ctx, jetStream)
}

func ensureDeadLetterStream(ctx context.Context, jetStream nats.JetStreamContext) error {
	desired := &nats.StreamConfig{
		Name: DeadLetterStreamName, Description: "Permanent document processing failures",
		Subjects: []string{DeadLetterSubject}, Retention: nats.LimitsPolicy,
		MaxAge: 30 * 24 * time.Hour, MaxBytes: 2 << 30, MaxMsgSize: 2 << 20,
		Storage: nats.FileStorage, Replicas: 1, Discard: nats.DiscardOld,
	}
	_, err := jetStream.StreamInfo(DeadLetterStreamName, nats.Context(ctx))
	if errors.Is(err, nats.ErrStreamNotFound) {
		_, err = jetStream.AddStream(desired, nats.Context(ctx))
		if err != nil {
			return fmt.Errorf("create NATS JetStream dead-letter stream: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect NATS JetStream dead-letter stream: %w", err)
	}
	if _, err := jetStream.UpdateStream(desired, nats.Context(ctx)); err != nil {
		return fmt.Errorf("update NATS JetStream dead-letter stream: %w", err)
	}
	return nil
}

func Publish(ctx context.Context, jetStream nats.JetStreamContext, subject, taskID, docID, processingMode string) error {
	payload, err := json.Marshal(task.Message{
		TaskID:         taskID,
		DocID:          docID,
		ProcessingMode: task.NormalizeProcessingMode(processingMode),
	})
	if err != nil {
		return err
	}
	return PublishPayload(ctx, jetStream, subject, taskID, payload)
}

func PublishPayload(ctx context.Context, jetStream nats.JetStreamContext, subject, taskID string, payload []byte) error {
	return PublishPayloadWithHeaders(ctx, jetStream, subject, taskID, payload, nil)
}

func PublishPayloadWithHeaders(ctx context.Context, jetStream nats.JetStreamContext, subject, taskID string, payload []byte, headers map[string]string) error {
	if len(headers) > 0 {
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(headers))
	}
	ctx, span := observability.StartSpan(ctx, "nats.publish",
		attribute.String("messaging.system", "nats"),
		attribute.String("messaging.destination", subject),
	)
	message := nats.NewMsg(subject)
	message.Data = payload
	injectContext(ctx, message)
	_, err := jetStream.PublishMsg(message, nats.Context(ctx), nats.MsgId(taskID))
	observability.EndSpan(span, err)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPublish, err)
	}
	return nil
}

func PublishDeadLetter(ctx context.Context, jetStream nats.JetStreamContext, taskType, taskID string, payload []byte, taskErr error) error {
	body, err := json.Marshal(map[string]any{
		"task_type": taskType,
		"task_id":   taskID,
		"payload":   string(payload),
		"error":     taskErr.Error(),
		"failed_at": time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	messageID := "dlq:" + taskID
	if taskID == "" {
		messageID = fmt.Sprintf("dlq:invalid:%d", time.Now().UnixNano())
	}
	return PublishPayload(ctx, jetStream, DeadLetterSubject, messageID, body)
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
