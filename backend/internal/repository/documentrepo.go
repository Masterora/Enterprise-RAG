package repository

import (
	"context"

	"enterprise-rag/backend/internal/model"
)

type DocumentRepository interface {
	CreateWithIndexTask(ctx context.Context, document *model.Document, task *model.IndexTask) error
	ListByUser(ctx context.Context, filter model.DocumentListFilter) ([]model.Document, int64, error)
}
