package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"enterprise-rag/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrIndexTaskNotRetryable = errors.New("index task is not retryable")

type IndexTaskRepo struct {
	db *pgxpool.Pool
}

func NewIndexTaskRepo(db *pgxpool.Pool) *IndexTaskRepo {
	return &IndexTaskRepo{db: db}
}

func (r *IndexTaskRepo) Create(ctx context.Context, task *model.IndexTask) error {
	_, err := r.db.Exec(
		ctx,
		`INSERT INTO index_tasks (id, doc_id, subject_id, user_id, task_type, status, retry_count, error_message, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)`,
		task.ID,
		task.DocID,
		task.SubjectID,
		task.UserID,
		task.TaskType,
		task.Status,
		task.RetryCount,
		task.ErrorMessage,
		task.Metadata,
		task.CreatedAt,
	)
	return err
}

func (r *IndexTaskRepo) GetByID(ctx context.Context, taskID string) (*model.IndexTask, error) {
	var task model.IndexTask
	var errorMessage sql.NullString
	err := r.db.QueryRow(
		ctx,
		`SELECT id::text, doc_id::text, subject_id::text, user_id::text,
		        task_type, status, retry_count, error_message, COALESCE(metadata, '{}'::jsonb), created_at, updated_at
		 FROM index_tasks
		 WHERE id = $1`,
		taskID,
	).Scan(
		&task.ID,
		&task.DocID,
		&task.SubjectID,
		&task.UserID,
		&task.TaskType,
		&task.Status,
		&task.RetryCount,
		&errorMessage,
		&task.Metadata,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	task.ErrorMessage = errorMessage.String
	return &task, nil
}

func (r *IndexTaskRepo) GetByIDForUser(ctx context.Context, taskID, userID string) (*model.IndexTask, error) {
	var task model.IndexTask
	var errorMessage sql.NullString
	err := r.db.QueryRow(
		ctx,
		`SELECT t.id::text, t.doc_id::text, t.subject_id::text, t.user_id::text,
		        COALESCE(d.filename, ''), t.task_type, t.status, t.retry_count,
		        t.error_message, COALESCE(t.metadata, '{}'::jsonb), t.created_at, t.updated_at
		 FROM index_tasks t
		 LEFT JOIN documents d ON d.id = t.doc_id
		 WHERE t.id = $1 AND t.user_id = $2`,
		taskID,
		userID,
	).Scan(
		&task.ID,
		&task.DocID,
		&task.SubjectID,
		&task.UserID,
		&task.Filename,
		&task.TaskType,
		&task.Status,
		&task.RetryCount,
		&errorMessage,
		&task.Metadata,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	task.ErrorMessage = errorMessage.String
	return &task, nil
}

func (r *IndexTaskRepo) List(ctx context.Context, filter model.IndexTaskListFilter) ([]model.IndexTask, int64, error) {
	conditions := []string{"t.user_id = $1", "t.cleared_at IS NULL"}
	args := []any{filter.UserID}
	if filter.SubjectID != "" {
		args = append(args, filter.SubjectID)
		conditions = append(conditions, fmt.Sprintf("t.subject_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("t.status = $%d", len(args)))
	}
	if filter.TaskType != "" {
		args = append(args, filter.TaskType)
		conditions = append(conditions, fmt.Sprintf("t.task_type = $%d", len(args)))
	}
	where := strings.Join(conditions, " AND ")

	var total int64
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM index_tasks t WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, filter.PageSize, filter.Offset)
	rows, err := r.db.Query(
		ctx,
		fmt.Sprintf(
			`SELECT t.id::text, t.doc_id::text, t.subject_id::text, t.user_id::text,
			        COALESCE(d.filename, ''), t.task_type, t.status, t.retry_count,
			        COALESCE(t.error_message, ''), COALESCE(t.metadata, '{}'::jsonb), t.created_at, t.updated_at
			 FROM index_tasks t
			 LEFT JOIN documents d ON d.id = t.doc_id
			 WHERE %s
			 ORDER BY t.created_at DESC
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

	tasks := make([]model.IndexTask, 0)
	for rows.Next() {
		var task model.IndexTask
		if err := rows.Scan(
			&task.ID,
			&task.DocID,
			&task.SubjectID,
			&task.UserID,
			&task.Filename,
			&task.TaskType,
			&task.Status,
			&task.RetryCount,
			&task.ErrorMessage,
			&task.Metadata,
			&task.CreatedAt,
			&task.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, task)
	}
	return tasks, total, rows.Err()
}

func (r *IndexTaskRepo) ClearTerminalByUser(ctx context.Context, userID string) (int64, error) {
	result, err := r.db.Exec(
		ctx,
		`UPDATE index_tasks
		 SET cleared_at = now(), updated_at = now()
		 WHERE user_id = $1
		   AND cleared_at IS NULL
		   AND status IN ($2, $3)`,
		userID,
		model.TaskStatusSuccess,
		model.TaskStatusFailed,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (r *IndexTaskRepo) ListByDocument(ctx context.Context, docID, userID string) ([]model.IndexTask, error) {
	rows, err := r.db.Query(
		ctx,
		`SELECT t.id::text, t.doc_id::text, t.subject_id::text, t.user_id::text,
		        COALESCE(d.filename, ''), t.task_type, t.status, t.retry_count,
		        COALESCE(t.error_message, ''), COALESCE(t.metadata, '{}'::jsonb), t.created_at, t.updated_at
		 FROM index_tasks t
		 LEFT JOIN documents d ON d.id = t.doc_id
		 WHERE t.doc_id = $1 AND t.user_id = $2
		 ORDER BY t.created_at ASC`,
		docID,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]model.IndexTask, 0)
	for rows.Next() {
		var task model.IndexTask
		if err := rows.Scan(
			&task.ID,
			&task.DocID,
			&task.SubjectID,
			&task.UserID,
			&task.Filename,
			&task.TaskType,
			&task.Status,
			&task.RetryCount,
			&task.ErrorMessage,
			&task.Metadata,
			&task.CreatedAt,
			&task.UpdatedAt,
		); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *IndexTaskRepo) UpdateStatus(ctx context.Context, taskID, status, errMsg string) error {
	_, err := r.db.Exec(
		ctx,
		`UPDATE index_tasks
		 SET status = $2, error_message = $3, updated_at = now()
		 WHERE id = $1`,
		taskID,
		status,
		errMsg,
	)
	return err
}

func (r *IndexTaskRepo) ScheduleRetry(ctx context.Context, taskID string, maxRetries int) (*model.IndexTask, bool, error) {
	var task model.IndexTask
	var errorMessage sql.NullString
	err := r.db.QueryRow(
		ctx,
		`UPDATE index_tasks
		 SET status = $2, retry_count = retry_count + 1, updated_at = now()
		 WHERE id = $1 AND retry_count < $3 AND status IN ($4, $5)
		 RETURNING id::text, doc_id::text, subject_id::text, user_id::text,
		           task_type, status, retry_count, error_message, COALESCE(metadata, '{}'::jsonb), created_at, updated_at`,
		taskID,
		model.TaskStatusPending,
		maxRetries,
		model.TaskStatusRunning,
		model.TaskStatusFailed,
	).Scan(
		&task.ID,
		&task.DocID,
		&task.SubjectID,
		&task.UserID,
		&task.TaskType,
		&task.Status,
		&task.RetryCount,
		&errorMessage,
		&task.Metadata,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	task.ErrorMessage = errorMessage.String
	return &task, true, nil
}

func (r *IndexTaskRepo) Retry(ctx context.Context, taskID, userID string) (*model.IndexTask, error) {
	var task model.IndexTask
	var errorMessage sql.NullString
	err := r.db.QueryRow(
		ctx,
		`UPDATE index_tasks
		 SET status = $3, retry_count = retry_count + 1, error_message = NULL, updated_at = now()
		 WHERE id = $1 AND user_id = $2 AND status = $4
		 RETURNING id::text, doc_id::text, subject_id::text, user_id::text,
		           task_type, status, retry_count, error_message, COALESCE(metadata, '{}'::jsonb), created_at, updated_at`,
		taskID,
		userID,
		model.TaskStatusPending,
		model.TaskStatusFailed,
	).Scan(
		&task.ID,
		&task.DocID,
		&task.SubjectID,
		&task.UserID,
		&task.TaskType,
		&task.Status,
		&task.RetryCount,
		&errorMessage,
		&task.Metadata,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrIndexTaskNotRetryable
		}
		return nil, err
	}
	task.ErrorMessage = errorMessage.String
	return &task, nil
}
