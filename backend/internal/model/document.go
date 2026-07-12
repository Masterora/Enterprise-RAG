package model

import "time"

type Document struct {
	ID           string
	SubjectID    string
	UserID       string
	Filename     string
	FileType     string
	FileSize     int64
	FileURL      string
	Status       string
	PlainText    string
	Metadata     []byte
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type DocumentListFilter struct {
	UserID    string
	SubjectID string
	Status    string
	Keyword   string
	PageSize  int
	Offset    int
}

type IndexTask struct {
	ID           string
	DocID        string
	SubjectID    string
	UserID       string
	Filename     string
	TaskType     string
	Status       string
	RetryCount   int
	Metadata     []byte
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type IndexTaskListFilter struct {
	UserID    string
	SubjectID string
	Status    string
	TaskType  string
	PageSize  int
	Offset    int
}
