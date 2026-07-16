package postgres

import (
	"context"

	"enterprise-rag/api/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminRepo struct {
	db *pgxpool.Pool
}

func NewAdminRepo(db *pgxpool.Pool) *AdminRepo {
	return &AdminRepo{db: db}
}

func (r *AdminRepo) Summary(ctx context.Context, userID string) (*model.AdminSummary, error) {
	var summary model.AdminSummary
	err := r.db.QueryRow(
		ctx,
		`SELECT
		   (SELECT count(*) FROM subjects WHERE owner_id = $1 AND deleted_at IS NULL),
		   (SELECT count(*) FROM documents WHERE user_id = $1 AND deleted_at IS NULL),
		   (SELECT count(*) FROM document_chunks WHERE user_id = $1 AND deleted_at IS NULL),
		   (SELECT count(*) FROM chat_sessions WHERE user_id = $1 AND deleted_at IS NULL),
		   (SELECT count(*) FROM documents WHERE user_id = $1 AND deleted_at IS NULL AND status = $2),
		   (SELECT count(*) FROM documents WHERE user_id = $1 AND deleted_at IS NULL AND status IN ($3, $4, $5, $6, $7, $8)),
		   (SELECT count(*) FROM documents WHERE user_id = $1 AND deleted_at IS NULL AND status IN ($9, $10)),
		   (SELECT count(*) FROM index_tasks WHERE user_id = $1 AND status = $11),
		   (SELECT count(*) FROM index_tasks WHERE user_id = $1 AND status = $12),
		   (SELECT count(*) FROM index_tasks WHERE user_id = $1 AND status = $13)`,
		userID,
		model.DocumentStatusIndexed,
		model.DocumentStatusUploaded,
		model.DocumentStatusParsing,
		model.DocumentStatusParsed,
		model.DocumentStatusChunking,
		model.DocumentStatusChunked,
		model.DocumentStatusEmbedding,
		model.DocumentStatusFailed,
		model.DocumentStatusDeleteFailed,
		model.TaskStatusPending,
		model.TaskStatusRunning,
		model.TaskStatusFailed,
	).Scan(
		&summary.SubjectTotal,
		&summary.DocumentTotal,
		&summary.ChunkTotal,
		&summary.SessionTotal,
		&summary.IndexedTotal,
		&summary.ProcessingTotal,
		&summary.FailedTotal,
		&summary.PendingTaskTotal,
		&summary.RunningTaskTotal,
		&summary.FailedTaskTotal,
	)
	if err != nil {
		return nil, err
	}
	return &summary, nil
}
