package model

import "time"

type Document struct {
	ID                   string
	TenantID             string
	SubjectID            string
	UserID               string
	Filename             string
	FileType             string
	FileSize             int64
	FileURL              string
	Status               string
	PlainText            string
	Metadata             []byte
	ErrorMessage         string
	DocumentVersion      int
	ContentHash          string
	EmbeddingProvider    string
	EmbeddingModel       string
	EmbeddingDimension   int
	ChunkStrategyVersion string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type DocumentListFilter struct {
	UserID         string
	TenantID       string
	AllTenantUsers bool
	SubjectID      string
	Status         string
	Keyword        string
	PageSize       int
	Offset         int
}

type IndexTask struct {
	ID                   string
	TenantID             string
	DocID                string
	SubjectID            string
	UserID               string
	Filename             string
	TaskType             string
	Status               string
	RetryCount           int
	Metadata             []byte
	ErrorMessage         string
	DocumentVersion      int
	ContentHash          string
	EmbeddingProvider    string
	EmbeddingModel       string
	EmbeddingDimension   int
	ChunkStrategyVersion string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type IndexTaskListFilter struct {
	UserID    string
	TenantID  string
	SubjectID string
	Status    string
	TaskType  string
	PageSize  int
	Offset    int
}
