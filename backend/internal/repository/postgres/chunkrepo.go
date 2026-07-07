package postgres

import (
	"context"

	"enterprise-rag/backend/internal/model"

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
			 (id, doc_id, subject_id, user_id, chunk_index, content, page, section, token_count, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)`,
			chunk.ID,
			chunk.DocID,
			chunk.SubjectID,
			chunk.UserID,
			chunk.ChunkIndex,
			chunk.Content,
			chunk.Page,
			chunk.Section,
			chunk.TokenCount,
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
		`SELECT id::text, doc_id::text, subject_id::text, user_id::text, chunk_index, content, COALESCE(page, 0), COALESCE(section, ''), COALESCE(token_count, 0), created_at, updated_at
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
			&chunk.DocID,
			&chunk.SubjectID,
			&chunk.UserID,
			&chunk.ChunkIndex,
			&chunk.Content,
			&chunk.Page,
			&chunk.Section,
			&chunk.TokenCount,
			&chunk.CreatedAt,
			&chunk.UpdatedAt,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

func (r *ChunkRepo) ListBySubject(ctx context.Context, subjectID string) ([]model.DocumentChunk, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT id::text, doc_id::text, subject_id::text, user_id::text, chunk_index, content, COALESCE(page, 0), COALESCE(section, ''), COALESCE(token_count, 0), created_at, updated_at
		 FROM document_chunks
		 WHERE subject_id = $1 AND deleted_at IS NULL
		 ORDER BY chunk_index ASC`,
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
			&chunk.DocID,
			&chunk.SubjectID,
			&chunk.UserID,
			&chunk.ChunkIndex,
			&chunk.Content,
			&chunk.Page,
			&chunk.Section,
			&chunk.TokenCount,
			&chunk.CreatedAt,
			&chunk.UpdatedAt,
		); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}
