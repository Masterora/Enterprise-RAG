package repository

import (
	"context"

	"enterprise-rag/backend/internal/model"
)

type AdminRepository interface {
	Summary(ctx context.Context, userID string) (*model.AdminSummary, error)
}
