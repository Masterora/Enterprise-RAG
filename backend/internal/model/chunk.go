package model

import "time"

type DocumentChunk struct {
	ID         string
	DocID      string
	SubjectID  string
	UserID     string
	ChunkIndex int
	Content    string
	Page       int
	Section    string
	TokenCount int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type ParseSegment struct {
	Page    int    `json:"page"`
	Section string `json:"section"`
	Content string `json:"content"`
}

type DocumentMetadata struct {
	Segments []ParseSegment `json:"segments"`
}

type DocumentParseLog struct {
	ID        string
	DocID     string
	Filename  string
	Status    string
	Message   string
	Error     string
	CreatedAt time.Time
}

type ParseLogListFilter struct {
	UserID    string
	DocID     string
	SubjectID string
	Status    string
	PageSize  int
	Offset    int
}
