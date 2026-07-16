package repository

import (
	"context"

	"enterprise-rag/api/internal/model"
)

type ChatRepository interface {
	CreateSession(ctx context.Context, session *model.ChatSession) error
	ListSessions(ctx context.Context, userID string) ([]model.ChatSession, error)
	UpdateSession(ctx context.Context, sessionID, userID, title string) (bool, error)
	DeleteSession(ctx context.Context, sessionID, userID string) (bool, error)
	SaveTurn(ctx context.Context, session *model.ChatSession, message *model.ChatMessage) error
}
