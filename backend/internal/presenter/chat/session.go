package chat

import (
	"encoding/json"
	"time"

	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/types"
)

type MessageMetadata struct {
	Metrics       types.RetrievalMetrics `json:"metrics"`
	ModelLabel    string                 `json:"model_label"`
	ModelID       string                 `json:"model_id"`
	WebSearch     bool                   `json:"web_search"`
	ExternalLinks []types.ExternalLink   `json:"external_links"`
	AgentSteps    []types.AgentStep      `json:"agent_steps"`
}

func MapSession(session model.ChatSession) types.ChatSessionInfo {
	messages := make([]types.ChatMessageInfo, 0, len(session.Messages))
	for _, message := range session.Messages {
		var chunks []types.RetrievalChunk
		var metadata MessageMetadata
		_ = json.Unmarshal(message.Citations, &chunks)
		_ = json.Unmarshal(message.Metadata, &metadata)

		messages = append(messages, types.ChatMessageInfo{
			ID:            message.ID,
			Question:      message.Question,
			Answer:        message.Answer,
			Chunks:        chunks,
			ExternalLinks: metadata.ExternalLinks,
			Metrics:       metadata.Metrics,
			ModelLabel:    metadata.ModelLabel,
			ModelID:       metadata.ModelID,
			WebSearch:     metadata.WebSearch,
			AgentSteps:    metadata.AgentSteps,
			CreatedAt:     message.CreatedAt.Format(time.RFC3339Nano),
		})
	}

	return types.ChatSessionInfo{
		ID:          session.ID,
		Title:       session.Title,
		SubjectID:   session.SubjectID,
		LlmProvider: session.LLMProvider,
		LlmModel:    session.LLMModel,
		CreatedAt:   session.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt:   session.UpdatedAt.Format(time.RFC3339Nano),
		Messages:    messages,
	}
}
