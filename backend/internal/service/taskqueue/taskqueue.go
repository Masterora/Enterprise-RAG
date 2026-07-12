package taskqueue

import (
	"encoding/json"

	"enterprise-rag/backend/internal/task"

	"github.com/nats-io/nats.go"
)

func Publish(connection *nats.Conn, subject, taskID, docID, processingMode string) error {
	payload, err := json.Marshal(task.Message{
		TaskID:         taskID,
		DocID:          docID,
		ProcessingMode: task.NormalizeProcessingMode(processingMode),
	})
	if err != nil {
		return err
	}
	return connection.Publish(subject, payload)
}
