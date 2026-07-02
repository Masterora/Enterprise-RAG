package model

import "time"

type Subject struct {
	ID          string
	Name        string
	Description string
	OwnerID     string
	Visibility  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SubjectListFilter struct {
	UserID   string
	Keyword  string
	PageSize int
	Offset   int
}
