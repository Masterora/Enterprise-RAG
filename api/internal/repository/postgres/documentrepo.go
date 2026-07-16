package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"enterprise-rag/api/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrDocumentStateConflict = errors.New("document state transition conflict")

type DocumentRepo struct {
	db *pgxpool.Pool
}

func NewDocumentRepo(db *pgxpool.Pool) *DocumentRepo {
	return &DocumentRepo{db: db}
}

func (r *DocumentRepo) ExistsActiveFilename(ctx context.Context, userID, subjectID, filename string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(
		ctx,
		`SELECT EXISTS (
			SELECT 1
			FROM documents d
			JOIN subjects s ON s.id = d.subject_id
			WHERE d.user_id = $1
			  AND d.subject_id = $2
			  AND d.filename = $3
			  AND d.deleted_at IS NULL
			  AND s.deleted_at IS NULL
		)`,
		userID,
		subjectID,
		filename,
	).Scan(&exists)
	return exists, err
}

func (r *DocumentRepo) CreateWithIndexTask(ctx context.Context, document *model.Document, task *model.IndexTask) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO documents (id, tenant_id, subject_id, user_id, filename, file_type, file_size, file_url, status,
		 document_version, content_hash, embedding_provider, embedding_model, embedding_dimension, chunk_strategy_version,
		 created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $16)`,
		document.ID,
		document.TenantID,
		document.SubjectID,
		document.UserID,
		document.Filename,
		document.FileType,
		document.FileSize,
		document.FileURL,
		document.Status,
		document.DocumentVersion,
		document.ContentHash,
		document.EmbeddingProvider,
		document.EmbeddingModel,
		document.EmbeddingDimension,
		document.ChunkStrategyVersion,
		document.CreatedAt,
	); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO index_tasks (id, tenant_id, doc_id, subject_id, user_id, task_type, status, retry_count,
		 error_message, metadata, document_version, content_hash, embedding_provider, embedding_model,
		 embedding_dimension, chunk_strategy_version, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $17)`,
		task.ID,
		task.TenantID,
		task.DocID,
		task.SubjectID,
		task.UserID,
		task.TaskType,
		task.Status,
		task.RetryCount,
		sql.NullString{String: task.ErrorMessage, Valid: task.ErrorMessage != ""},
		task.Metadata,
		task.DocumentVersion,
		task.ContentHash,
		task.EmbeddingProvider,
		task.EmbeddingModel,
		task.EmbeddingDimension,
		task.ChunkStrategyVersion,
		task.CreatedAt,
	); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if err := enqueueOutbox(ctx, tx, task, false); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *DocumentRepo) ListByUser(ctx context.Context, filter model.DocumentListFilter) ([]model.Document, int64, error) {
	conditions := []string{"d.deleted_at IS NULL", "d.tenant_id = $1", "s.deleted_at IS NULL"}
	args := []any{filter.TenantID}
	if !filter.AllTenantUsers {
		args = append(args, filter.UserID)
		conditions = append(conditions, fmt.Sprintf("d.user_id = $%d", len(args)))
	}

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
		fmt.Sprintf(`SELECT d.id::text, d.subject_id::text, d.user_id::text, d.filename, d.file_type, d.file_size, d.file_url, d.status, COALESCE(d.error_message, ''), d.created_at, d.updated_at
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
			&document.ErrorMessage,
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
		`SELECT id::text, tenant_id::text, subject_id::text, user_id::text, filename, file_type, file_size, file_url, status,
		        plain_text, metadata, error_message, document_version, content_hash, embedding_provider,
		        embedding_model, embedding_dimension, chunk_strategy_version, created_at, updated_at
		 FROM documents
		 WHERE id = $1 AND deleted_at IS NULL`,
		docID,
	).Scan(
		&document.ID,
		&document.TenantID,
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
		&document.DocumentVersion,
		&document.ContentHash,
		&document.EmbeddingProvider,
		&document.EmbeddingModel,
		&document.EmbeddingDimension,
		&document.ChunkStrategyVersion,
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

func (r *DocumentRepo) GetByIDForUser(ctx context.Context, docID, userID string) (*model.Document, error) {
	document, err := r.GetByID(ctx, docID)
	if err != nil {
		return nil, err
	}
	if document.UserID != userID {
		return nil, sql.ErrNoRows
	}
	return document, nil
}

func (r *DocumentRepo) CompleteParse(ctx context.Context, docID, plainText string, metadata []byte) error {
	result, err := r.db.Exec(
		ctx,
		`UPDATE documents
		 SET status = $2, plain_text = $3, metadata = $4, error_message = NULL, updated_at = now()
		 WHERE id = $1 AND deleted_at IS NULL AND status = $5`,
		docID,
		model.DocumentStatusParsed,
		sql.NullString{String: plainText, Valid: plainText != ""},
		metadata,
		model.DocumentStatusParsing,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrDocumentStateConflict
	}
	return nil
}

func (r *DocumentRepo) UpdateStatus(ctx context.Context, docID, status, errMsg string) error {
	previous, ok := model.DocumentPreviousStatuses(status)
	if !ok {
		return ErrDocumentStateConflict
	}
	result, err := r.db.Exec(
		ctx,
		`UPDATE documents
		 SET status = $2, error_message = $3, updated_at = now()
		 WHERE id = $1 AND deleted_at IS NULL AND status = ANY($4)`,
		docID,
		status,
		sql.NullString{String: errMsg, Valid: errMsg != ""},
		previous,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrDocumentStateConflict
	}
	return nil
}

func (r *DocumentRepo) ResetStatusForRetry(ctx context.Context, docID, status string) error {
	previous, ok := model.DocumentRetryPreviousStatuses(status)
	if !ok {
		return ErrDocumentStateConflict
	}
	result, err := r.db.Exec(
		ctx,
		`UPDATE documents
		 SET status = $2, error_message = NULL, updated_at = now()
		 WHERE id = $1 AND deleted_at IS NULL AND status = ANY($3)`,
		docID,
		status,
		previous,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrDocumentStateConflict
	}
	return nil
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

func (r *DocumentRepo) ListParseLogs(ctx context.Context, filter model.ParseLogListFilter) ([]model.DocumentParseLog, int64, error) {
	conditions := []string{"d.tenant_id = $1", "d.user_id = $2"}
	args := []any{filter.TenantID, filter.UserID}
	if filter.SubjectID != "" {
		args = append(args, filter.SubjectID)
		conditions = append(conditions, fmt.Sprintf("d.subject_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("l.status = $%d", len(args)))
	}
	if filter.DocID != "" {
		args = append(args, filter.DocID)
		conditions = append(conditions, fmt.Sprintf("l.doc_id = $%d", len(args)))
	}
	where := strings.Join(conditions, " AND ")

	var total int64
	if err := r.db.QueryRow(
		ctx,
		"SELECT count(*) FROM document_parse_logs l JOIN documents d ON d.id = l.doc_id WHERE "+where,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, filter.PageSize, filter.Offset)
	rows, err := r.db.Query(
		ctx,
		fmt.Sprintf(
			`SELECT l.id::text, l.doc_id::text, d.filename, l.status,
			        COALESCE(l.message, ''), COALESCE(l.error_message, ''), l.created_at
			 FROM document_parse_logs l
			 JOIN documents d ON d.id = l.doc_id
			 WHERE %s
			 ORDER BY l.created_at DESC
			 LIMIT $%d OFFSET $%d`,
			where,
			len(args)-1,
			len(args),
		),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	logs := make([]model.DocumentParseLog, 0)
	for rows.Next() {
		var item model.DocumentParseLog
		if err := rows.Scan(
			&item.ID,
			&item.DocID,
			&item.Filename,
			&item.Status,
			&item.Message,
			&item.Error,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		logs = append(logs, item)
	}
	return logs, total, rows.Err()
}

func (r *DocumentRepo) ClearParseLogsByUser(ctx context.Context, userID, subjectID string) (int64, error) {
	conditions := []string{"d.user_id = $1"}
	args := []any{userID}
	if subjectID != "" {
		args = append(args, subjectID)
		conditions = append(conditions, fmt.Sprintf("d.subject_id = $%d", len(args)))
	}
	where := strings.Join(conditions, " AND ")

	result, err := r.db.Exec(
		ctx,
		"DELETE FROM document_parse_logs l USING documents d WHERE l.doc_id = d.id AND "+where,
		args...,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r *DocumentRepo) CreateDeleteTask(ctx context.Context, docID, userID string, task *model.IndexTask) error {
	deletePrevious, ok := model.DocumentPreviousStatuses(model.DocumentStatusDeleting)
	if !ok {
		return ErrDocumentStateConflict
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	result, err := tx.Exec(
		ctx,
		`UPDATE documents
		 SET status = $3, error_message = NULL, updated_at = now()
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL AND status = ANY($4)`,
		docID,
		userID,
		model.DocumentStatusDeleting,
		deletePrevious,
	)
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if result.RowsAffected() == 0 {
		_ = tx.Rollback(ctx)
		return sql.ErrNoRows
	}
	if _, err := tx.Exec(
		ctx,
		`INSERT INTO index_tasks
		 (id, tenant_id, doc_id, subject_id, user_id, task_type, status, retry_count,
		  document_version, content_hash, embedding_provider, embedding_model, embedding_dimension,
		  chunk_strategy_version, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 0, $8, $9, $10, $11, $12, $13, $14, $14)`,
		task.ID,
		task.TenantID,
		task.DocID,
		task.SubjectID,
		task.UserID,
		task.TaskType,
		task.Status,
		task.DocumentVersion,
		task.ContentHash,
		task.EmbeddingProvider,
		task.EmbeddingModel,
		task.EmbeddingDimension,
		task.ChunkStrategyVersion,
		task.CreatedAt,
	); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := enqueueOutbox(ctx, tx, task, false); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *DocumentRepo) CompleteDelete(ctx context.Context, docID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		ctx,
		`UPDATE document_chunks
		 SET deleted_at = now(), updated_at = now()
		 WHERE doc_id = $1 AND deleted_at IS NULL`,
		docID,
	); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	result, err := tx.Exec(
		ctx,
		`UPDATE documents
		 SET deleted_at = now(), updated_at = now()
		 WHERE id = $1 AND deleted_at IS NULL AND status = $2`,
		docID,
		model.DocumentStatusDeleting,
	)
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if result.RowsAffected() == 0 {
		_ = tx.Rollback(ctx)
		return ErrDocumentStateConflict
	}
	return tx.Commit(ctx)
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
