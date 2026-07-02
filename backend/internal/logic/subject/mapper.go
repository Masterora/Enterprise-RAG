package subject

import (
	"time"

	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/types"
)

func formatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

func toSubjectInfo(subject model.Subject) types.SubjectInfo {
	return types.SubjectInfo{
		ID:          subject.ID,
		Name:        subject.Name,
		Description: subject.Description,
		Visibility:  subject.Visibility,
		CreatedAt:   formatTime(subject.CreatedAt),
		UpdatedAt:   formatTime(subject.UpdatedAt),
	}
}
