package repository

import (
	"context"

	"enterprise-rag/api/internal/model"
)

type RunRepository interface {
	Create(ctx context.Context, run *model.ChatRun) error
	GetForUser(ctx context.Context, runID, tenantID, userID string) (*model.ChatRun, error)
	MarkRunning(ctx context.Context, runID, tenantID, userID string) error
	Complete(ctx context.Context, runID, tenantID, userID string, result []byte) error
	Fail(ctx context.Context, runID, tenantID, userID, status, message string) error
	RequestCancel(ctx context.Context, runID, tenantID, userID string) (bool, error)
	ResetForResume(ctx context.Context, runID, tenantID, userID string) error
	AppendEvent(ctx context.Context, event *model.ChatRunEvent) (int64, error)
	ListEvents(ctx context.Context, runID, tenantID, userID string, afterSequence int64, limit int) ([]model.ChatRunEvent, error)
}
