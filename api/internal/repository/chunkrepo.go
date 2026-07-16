package repository

import (
	"context"

	"enterprise-rag/api/internal/model"
)

type ChunkRepository interface {
	ReplaceByDocument(ctx context.Context, chunks []model.DocumentChunk) error
	ListByDocument(ctx context.Context, docID string) ([]model.DocumentChunk, error)
	ListBySubject(ctx context.Context, tenantID, subjectID string) ([]model.DocumentChunk, error)
}
