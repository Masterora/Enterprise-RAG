package repository

import (
	"context"

	"enterprise-rag/backend/internal/model"
)

type IndexTaskRepository interface {
	Create(ctx context.Context, task *model.IndexTask) error
	GetByID(ctx context.Context, taskID string) (*model.IndexTask, error)
	GetByIDForUser(ctx context.Context, taskID, userID string) (*model.IndexTask, error)
	List(ctx context.Context, filter model.IndexTaskListFilter) ([]model.IndexTask, int64, error)
	ListByDocument(ctx context.Context, docID, userID string) ([]model.IndexTask, error)
	ClearTerminalByUser(ctx context.Context, userID string) (int64, error)
	UpdateStatus(ctx context.Context, taskID, status, errMsg string) error
	ScheduleRetry(ctx context.Context, taskID string, maxRetries int) (*model.IndexTask, bool, error)
	Retry(ctx context.Context, taskID, userID string) (*model.IndexTask, error)
}
