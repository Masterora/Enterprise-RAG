package repository

import (
	"context"

	"enterprise-rag/backend/internal/model"
)

type ChunkRepository interface {
	ReplaceByDocument(ctx context.Context, chunks []model.DocumentChunk) error
	ListByDocument(ctx context.Context, docID string) ([]model.DocumentChunk, error)
}
