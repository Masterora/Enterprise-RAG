package repository

import (
	"context"

	"enterprise-rag/backend/internal/model"
)

type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	GetByID(ctx context.Context, userID string) (*model.User, error)
	Create(ctx context.Context, user *model.User) error
}
