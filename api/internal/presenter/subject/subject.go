package subject

import (
	"time"

	"enterprise-rag/api/internal/model"
	"enterprise-rag/api/internal/types"
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
