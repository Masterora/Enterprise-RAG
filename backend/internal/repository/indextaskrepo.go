package repository

import (
	"context"

	"enterprise-rag/backend/internal/model"
)

type IndexTaskRepository interface {
	Create(ctx context.Context, task *model.IndexTask) error
	UpdateStatus(ctx context.Context, docID, taskType, status, errMsg string) error
}
