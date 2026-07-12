package subject

import (
	"time"

	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/types"
)

func ToInfo(subject model.Subject) types.SubjectInfo {
	return types.SubjectInfo{
		ID:          subject.ID,
		Name:        subject.Name,
		Description: subject.Description,
		Visibility:  subject.Visibility,
		CreatedAt:   subject.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   subject.UpdatedAt.Format(time.RFC3339),
	}
}
