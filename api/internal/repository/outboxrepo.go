package repository

import (
	"context"

	"enterprise-rag/api/internal/model"
)

type OutboxRepository interface {
	ClaimBatch(ctx context.Context, limit int) ([]model.TaskOutbox, error)
	MarkPublished(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id, message string) error
}
