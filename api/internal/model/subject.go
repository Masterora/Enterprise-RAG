package model

import "time"

type Subject struct {
	ID          string
	TenantID    string
	Name        string
	Description string
	OwnerID     string
	Visibility  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SubjectListFilter struct {
	UserID   string
	TenantID string
	Keyword  string
	PageSize int
	Offset   int
}
