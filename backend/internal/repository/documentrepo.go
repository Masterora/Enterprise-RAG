package repository

import (
	"context"

	"enterprise-rag/backend/internal/model"
)

type DocumentRepository interface {
	CreateWithIndexTask(ctx context.Context, document *model.Document, task *model.IndexTask) error
	ListByUser(ctx context.Context, filter model.DocumentListFilter) ([]model.Document, int64, error)
	GetByID(ctx context.Context, docID string) (*model.Document, error)
	UpdateParseResult(ctx context.Context, docID, status, plainText string, metadata []byte, errMsg string) error
	UpdateStatus(ctx context.Context, docID, status, errMsg string) error
	AddParseLog(ctx context.Context, log *model.DocumentParseLog) error
	ListActiveFailedDocIDsByUser(ctx context.Context, userID string) ([]string, error)
	ClearFailedByUser(ctx context.Context, userID string) (int64, error)
}
