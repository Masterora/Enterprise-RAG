package model

import (
	"encoding/json"
	"time"
)

type ChatSession struct {
	ID          string
	UserID      string
	SubjectID   string
	Title       string
	LLMProvider string
	LLMModel    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Messages    []ChatMessage
}

type ChatMessage struct {
	ID        string
	SessionID string
	UserID    string
	Question  string
	Answer    string
	Citations json.RawMessage
	Metadata  json.RawMessage
	CreatedAt time.Time
}
