package chat

import (
	"enterprise-rag/api/internal/model"
	"enterprise-rag/api/internal/types"
)

const timeLayout = "2006-01-02T15:04:05Z07:00"

func mapRunInfo(run *model.ChatRun) types.ChatRunInfo {
	return types.ChatRunInfo{
		ID: run.ID, Status: run.Status, SubjectID: run.SubjectID, SessionID: run.SessionID,
		MessageID: run.MessageID, ErrorMessage: run.ErrorMessage, CancelRequested: run.CancelRequested,
		CreatedAt: run.CreatedAt.Format(timeLayout), UpdatedAt: run.UpdatedAt.Format(timeLayout),
	}
}
