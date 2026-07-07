package postgres

import (
	"context"
	"database/sql"
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
		`INSERT INTO index_tasks (id, doc_id, subject_id, user_id, task_type, status, retry_count, error_message, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`,
		task.ID,
		task.DocID,
		task.SubjectID,
		task.UserID,
		task.TaskType,
		task.Status,
		task.RetryCount,
		sql.NullString{String: task.ErrorMessage, Valid: task.ErrorMessage != ""},
		task.CreatedAt,
	); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (r *DocumentRepo) ListByUser(ctx context.Context, filter model.DocumentListFilter) ([]model.Document, int64, error) {
	conditions := []string{"d.deleted_at IS NULL", "d.user_id = $1", "s.deleted_at IS NULL"}
	args := []any{filter.UserID}

	if filter.SubjectID != "" {
		args = append(args, filter.SubjectID)
		conditions = append(conditions, fmt.Sprintf("d.subject_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("d.status = $%d", len(args)))
	}
	if filter.Keyword != "" {
		args = append(args, "%"+filter.Keyword+"%")
		conditions = append(conditions, fmt.Sprintf("d.filename ILIKE $%d", len(args)))
	}

	where := strings.Join(conditions, " AND ")

	var total int64
	if err := r.db.QueryRow(
		ctx,
		"SELECT count(*) FROM documents d JOIN subjects s ON s.id = d.subject_id WHERE "+where,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, filter.PageSize, filter.Offset)
	rows, err := r.db.Query(
		ctx,
		fmt.Sprintf(`SELECT d.id::text, d.subject_id::text, d.user_id::text, d.filename, d.file_type, d.file_size, d.file_url, d.status, d.created_at, d.updated_at
			FROM documents d
			JOIN subjects s ON s.id = d.subject_id
			WHERE %s
			ORDER BY d.created_at DESC
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

func (r *DocumentRepo) GetByID(ctx context.Context, docID string) (*model.Document, error) {
	var (
		document     model.Document
		plainText    sql.NullString
		metadata     []byte
		errorMessage sql.NullString
	)
	err := r.db.QueryRow(
		ctx,
		`SELECT id::text, subject_id::text, user_id::text, filename, file_type, file_size, file_url, status,
		        plain_text, metadata, error_message, created_at, updated_at
		 FROM documents
		 WHERE id = $1 AND deleted_at IS NULL`,
		docID,
	).Scan(
		&document.ID,
		&document.SubjectID,
		&document.UserID,
		&document.Filename,
		&document.FileType,
		&document.FileSize,
		&document.FileURL,
		&document.Status,
		&plainText,
		&metadata,
		&errorMessage,
		&document.CreatedAt,
		&document.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	document.PlainText = plainText.String
	document.Metadata = metadata
	document.ErrorMessage = errorMessage.String
	return &document, nil
}

func (r *DocumentRepo) UpdateParseResult(ctx context.Context, docID, status, plainText string, metadata []byte, errMsg string) error {
	_, err := r.db.Exec(
		ctx,
		`UPDATE documents
		 SET status = $2, plain_text = $3, metadata = $4, error_message = $5, updated_at = now()
		 WHERE id = $1`,
		docID,
		status,
		sql.NullString{String: plainText, Valid: plainText != ""},
		metadata,
		sql.NullString{String: errMsg, Valid: errMsg != ""},
	)
	return err
}

func (r *DocumentRepo) UpdateStatus(ctx context.Context, docID, status, errMsg string) error {
	_, err := r.db.Exec(
		ctx,
		`UPDATE documents
		 SET status = $2, error_message = $3, updated_at = now()
		 WHERE id = $1`,
		docID,
		status,
		sql.NullString{String: errMsg, Valid: errMsg != ""},
	)
	return err
}

func (r *DocumentRepo) AddParseLog(ctx context.Context, log *model.DocumentParseLog) error {
	_, err := r.db.Exec(
		ctx,
		`INSERT INTO document_parse_logs (id, doc_id, status, message, error_message, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		log.ID,
		log.DocID,
		log.Status,
		sql.NullString{String: log.Message, Valid: log.Message != ""},
		sql.NullString{String: log.Error, Valid: log.Error != ""},
		log.CreatedAt,
	)
	return err
}

func (r *DocumentRepo) ListActiveFailedDocIDsByUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT id::text
		 FROM documents
		 WHERE user_id = $1 AND status = $2 AND deleted_at IS NULL`,
		userID,
		model.DocumentStatusFailed,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return ids, nil
}

func (r *DocumentRepo) ClearFailedByUser(ctx context.Context, userID string) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}

	if _, err := tx.Exec(
		ctx,
		`UPDATE document_chunks
		 SET deleted_at = now(), updated_at = now()
		 WHERE doc_id IN (
		   SELECT id
		   FROM documents
		   WHERE user_id = $1 AND status = $2 AND deleted_at IS NULL
		 )
		 AND deleted_at IS NULL`,
		userID,
		model.DocumentStatusFailed,
	); err != nil {
		_ = tx.Rollback(ctx)
		return 0, err
	}

	result, err := tx.Exec(
		ctx,
		`UPDATE documents
		 SET deleted_at = now(), updated_at = now()
		 WHERE user_id = $1 AND status = $2 AND deleted_at IS NULL`,
		userID,
		model.DocumentStatusFailed,
	)
	if err != nil {
		_ = tx.Rollback(ctx)
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
