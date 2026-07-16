package repository

import (
	"context"

	"enterprise-rag/api/internal/model"
)

type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	GetByID(ctx context.Context, userID string) (*model.User, error)
	Create(ctx context.Context, user *model.User) error
	UpdateProfile(ctx context.Context, userID, nickname, email, language string) (*model.User, error)
	UpdatePassword(ctx context.Context, userID, passwordHash string) error
}
