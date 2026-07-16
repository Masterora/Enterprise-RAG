package model

import (
	"encoding/json"
	"time"
)

const (
	RunStatusCreated   = "created"
	RunStatusRunning   = "running"
	RunStatusCompleted = "completed"
	RunStatusFailed    = "failed"
	RunStatusCancelled = "cancelled"
)

type ChatRun struct {
	ID              string
	TenantID        string
	UserID          string
	SessionID       string
	MessageID       string
	SubjectID       string
	Status          string
	Request         json.RawMessage
	Result          json.RawMessage
	ErrorMessage    string
	CancelRequested bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time
}

type ChatRunEvent struct {
	RunID     string
	Sequence  int64
	TenantID  string
	Type      string
	Payload   json.RawMessage
	CreatedAt time.Time
}
