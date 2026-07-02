package postgres

import (
	"context"
	"fmt"
	"strings"

	"enterprise-rag/backend/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DocumentRepo struct {
	db *pgxpool.Pool
}

func NewDocumentRepo(db *pgxpool.Pool) *DocumentRepo {
	return &DocumentRepo{db: db}
}

func (r *DocumentRepo) CreateWithIndexTask(ctx context.Context, document *model.Document, task *model.IndexTask) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO documents (id, subject_id, user_id, filename, file_type, file_size, file_url, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`,
		document.ID,
		document.SubjectID,
		document.UserID,
		document.Filename,
		document.FileType,
		document.FileSize,
		document.FileURL,
		document.Status,
		document.CreatedAt,
	); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO index_tasks (id, doc_id, subject_id, user_id, task_type, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		task.ID,
		task.DocID,
		task.SubjectID,
		task.UserID,
		task.TaskType,
		task.Status,
		task.CreatedAt,
		task.UpdatedAt,
	); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (r *DocumentRepo) ListByUser(ctx context.Context, filter model.DocumentListFilter) ([]model.Document, int64, error) {
	conditions := []string{"deleted_at IS NULL", "user_id = $1"}
	args := []any{filter.UserID}

	if filter.SubjectID != "" {
		args = append(args, filter.SubjectID)
		conditions = append(conditions, fmt.Sprintf("subject_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Keyword != "" {
		args = append(args, "%"+filter.Keyword+"%")
		conditions = append(conditions, fmt.Sprintf("filename ILIKE $%d", len(args)))
	}

	where := strings.Join(conditions, " AND ")

	var total int64
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM documents WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, filter.PageSize, filter.Offset)
	rows, err := r.db.Query(
		ctx,
		fmt.Sprintf(`SELECT id::text, subject_id::text, user_id::text, filename, file_type, file_size, file_url, status, created_at, updated_at
			FROM documents
			WHERE %s
			ORDER BY created_at DESC
			LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	documents := make([]model.Document, 0)
	for rows.Next() {
		var document model.Document
		if err := rows.Scan(
			&document.ID,
			&document.SubjectID,
			&document.UserID,
			&document.Filename,
			&document.FileType,
			&document.FileSize,
			&document.FileURL,
			&document.Status,
			&document.CreatedAt,
			&document.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		documents = append(documents, document)
	}
	if rows.Err() != nil {
		return nil, 0, rows.Err()
	}

	return documents, total, nil
}
