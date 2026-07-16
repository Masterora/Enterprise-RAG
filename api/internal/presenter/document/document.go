package document

import (
	"time"

	"enterprise-rag/api/internal/model"
	"enterprise-rag/api/internal/types"
)

func ToInfo(document model.Document) types.DocumentInfo {
	return types.DocumentInfo{
		ID:           document.ID,
		SubjectID:    document.SubjectID,
		Filename:     document.Filename,
		FileType:     document.FileType,
		FileSize:     document.FileSize,
		FileURL:      document.FileURL,
		Status:       document.Status,
		ErrorMessage: document.ErrorMessage,
		Progress:     progress(document.Status),
		CreatedAt:    document.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    document.UpdatedAt.Format(time.RFC3339),
	}
}

func ChunkToInfo(chunk model.DocumentChunk) types.DocumentChunkInfo {
	return types.DocumentChunkInfo{
		ID:         chunk.ID,
		ChunkIndex: chunk.ChunkIndex,
		Page:       chunk.Page,
		Section:    chunk.Section,
		Content:    chunk.Content,
		TokenCount: chunk.TokenCount,
	}
}

func TaskToInfo(task model.IndexTask) types.IndexTaskInfo {
	return types.IndexTaskInfo{
		ID:           task.ID,
		DocID:        task.DocID,
		SubjectID:    task.SubjectID,
		Filename:     task.Filename,
		TaskType:     task.TaskType,
		Status:       task.Status,
		RetryCount:   task.RetryCount,
		ErrorMessage: task.ErrorMessage,
		CreatedAt:    task.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    task.UpdatedAt.Format(time.RFC3339),
	}
}

func LogToInfo(item model.DocumentParseLog) types.ParseLogInfo {
	return types.ParseLogInfo{
		ID:           item.ID,
		DocID:        item.DocID,
		Filename:     item.Filename,
		Status:       item.Status,
		Message:      item.Message,
		ErrorMessage: item.Error,
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
	}
}

func progress(status string) int {
	switch status {
	case model.DocumentStatusUploaded:
		return 10
	case model.DocumentStatusParsing:
		return 25
	case model.DocumentStatusParsed:
		return 40
	case model.DocumentStatusChunking:
		return 50
	case model.DocumentStatusChunked:
		return 65
	case model.DocumentStatusEmbedding:
		return 80
	case model.DocumentStatusIndexed:
		return 100
	case model.DocumentStatusDeleting:
		return 50
	default:
		return 0
	}
}
