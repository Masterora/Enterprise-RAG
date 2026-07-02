package postgres

import (
	"context"

	"enterprise-rag/backend/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type IndexTaskRepo struct {
	db *pgxpool.Pool
}

func NewIndexTaskRepo(db *pgxpool.Pool) *IndexTaskRepo {
	return &IndexTaskRepo{db: db}
}

func (r *IndexTaskRepo) Create(ctx context.Context, task *model.IndexTask) error {
	_, err := r.db.Exec(
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
		task.ErrorMessage,
		task.CreatedAt,
	)
	return err
}

func (r *IndexTaskRepo) UpdateStatus(ctx context.Context, docID, taskType, status, errMsg string) error {
	_, err := r.db.Exec(
		ctx,
		`UPDATE index_tasks
		 SET status = $3, error_message = $4, updated_at = now()
		 WHERE doc_id = $1 AND task_type = $2`,
		docID,
		taskType,
		status,
		errMsg,
	)
	return err
}
