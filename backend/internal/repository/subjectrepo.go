package repository

import (
	"context"

	"enterprise-rag/backend/internal/model"
)

type SubjectRepository interface {
	Create(ctx context.Context, subject *model.Subject) error
	GetAccessibleByID(ctx context.Context, subjectID, userID string) (*model.Subject, error)
	ListAccessible(ctx context.Context, filter model.SubjectListFilter) ([]model.Subject, int64, error)
	ExistsAccessible(ctx context.Context, subjectID, userID string) (bool, error)
	UpdateByOwner(ctx context.Context, subject *model.Subject) error
	SoftDeleteByOwner(ctx context.Context, subjectID, userID string) (bool, error)
}
