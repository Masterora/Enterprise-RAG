package document

import (
	"time"

	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/types"
)

func formatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

func toDocumentInfo(document model.Document) types.DocumentInfo {
	return types.DocumentInfo{
		ID:        document.ID,
		SubjectID: document.SubjectID,
		Filename:  document.Filename,
		FileType:  document.FileType,
		FileSize:  document.FileSize,
		FileURL:   document.FileURL,
		Status:    document.Status,
		CreatedAt: formatTime(document.CreatedAt),
		UpdatedAt: formatTime(document.UpdatedAt),
	}
}
