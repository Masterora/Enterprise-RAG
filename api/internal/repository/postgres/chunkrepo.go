package postgres

import (
	"context"

	"enterprise-rag/api/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ChunkRepo struct {
	db *pgxpool.Pool
}

func NewChunkRepo(db *pgxpool.Pool) *ChunkRepo {
	return &ChunkRepo{db: db}
}

func (r *ChunkRepo) ReplaceByDocument(ctx context.Context, chunks []model.DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM document_chunks WHERE doc_id = $1`, chunks[0].DocID); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	for _, chunk := range chunks {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO document_chunks
				 (id, tenant_id, doc_id, subject_id, user_id, chunk_index, content, page, section, metadata,
				  token_count, document_version, content_hash, embedding_provider, embedding_model,
				  embedding_dimension, chunk_strategy_version, index_version, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $12, $18, $18)`,
			chunk.ID,
			chunk.TenantID,
			chunk.DocID,
			chunk.SubjectID,
			chunk.UserID,
			chunk.ChunkIndex,
			chunk.Content,
			chunk.Page,
			chunk.Section,
			chunk.Metadata,
			chunk.TokenCount,
			chunk.DocumentVersion,
			chunk.ContentHash,
			chunk.EmbeddingProvider,
			chunk.EmbeddingModel,
			chunk.EmbeddingDimension,
			chunk.ChunkStrategyVersion,
			chunk.CreatedAt,
		); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *ChunkRepo) ListByDocument(ctx context.Context, docID string) ([]model.DocumentChunk, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT id::text, tenant_id::text, doc_id::text, subject_id::text, user_id::text, chunk_index, content, COALESCE(page, 0), COALESCE(section, ''), COALESCE(metadata, '{}'::jsonb), COALESCE(token_count, 0),
		 document_version, content_hash, embedding_provider, embedding_model, embedding_dimension, chunk_strategy_version, created_at, updated_at
		 FROM document_chunks
		 WHERE doc_id = $1 AND deleted_at IS NULL
		 ORDER BY chunk_index ASC`,
		docID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chunks := make([]model.DocumentChunk, 0)
	for rows.Next() {
		var chunk model.DocumentChunk
		if err := rows.Scan(
			&chunk.ID,
			&chunk.TenantID,
			&chunk.DocID,
			&chunk.SubjectID,
			&chunk.UserID,
			&chunk.ChunkIndex,
			&chunk.Content,
			&chunk.Page,
			&chunk.Section,
			&chunk.Metadata,
			&chunk.TokenCount,
			&chunk.DocumentVersion,
			&chunk.ContentHash,
			&chunk.EmbeddingProvider,
			&chunk.EmbeddingModel,
			&chunk.EmbeddingDimension,
			&chunk.ChunkStrategyVersion,
			&chunk.CreatedAt,
			&chunk.UpdatedAt,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

func (r *ChunkRepo) ListBySubject(ctx context.Context, tenantID, subjectID string) ([]model.DocumentChunk, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT id::text, tenant_id::text, doc_id::text, subject_id::text, user_id::text, chunk_index, content, COALESCE(page, 0), COALESCE(section, ''), COALESCE(metadata, '{}'::jsonb), COALESCE(token_count, 0),
		 document_version, content_hash, embedding_provider, embedding_model, embedding_dimension, chunk_strategy_version, created_at, updated_at
		 FROM document_chunks
		 WHERE tenant_id = $1 AND subject_id = $2 AND deleted_at IS NULL
		 ORDER BY chunk_index ASC`,
		tenantID,
		subjectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chunks := make([]model.DocumentChunk, 0)
	for rows.Next() {
		var chunk model.DocumentChunk
		if err := rows.Scan(
			&chunk.ID,
			&chunk.TenantID,
			&chunk.DocID,
			&chunk.SubjectID,
			&chunk.UserID,
			&chunk.ChunkIndex,
			&chunk.Content,
			&chunk.Page,
			&chunk.Section,
			&chunk.Metadata,
			&chunk.TokenCount,
			&chunk.DocumentVersion,
			&chunk.ContentHash,
			&chunk.EmbeddingProvider,
			&chunk.EmbeddingModel,
			&chunk.EmbeddingDimension,
			&chunk.ChunkStrategyVersion,
			&chunk.CreatedAt,
			&chunk.UpdatedAt,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}
