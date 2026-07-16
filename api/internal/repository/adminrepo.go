package repository

import (
	"context"

	"enterprise-rag/api/internal/model"
)

type AdminRepository interface {
	Summary(ctx context.Context, userID string) (*model.AdminSummary, error)
}
