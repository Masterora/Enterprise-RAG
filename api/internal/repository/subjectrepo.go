package repository

import (
	"context"
	"errors"

	"enterprise-rag/api/internal/model"
)

var ErrSubjectNameExists = errors.New("subject name already exists")

type SubjectRepository interface {
	Create(ctx context.Context, subject *model.Subject) error
	GetAccessibleByID(ctx context.Context, subjectID, userID, tenantID string) (*model.Subject, error)
	ListAccessible(ctx context.Context, filter model.SubjectListFilter) ([]model.Subject, int64, error)
	ExistsAccessible(ctx context.Context, subjectID, userID, tenantID string) (bool, error)
	UpdateByOwner(ctx context.Context, subject *model.Subject) error
	SoftDeleteByOwner(ctx context.Context, subjectID, userID, tenantID string) (bool, error)
}
