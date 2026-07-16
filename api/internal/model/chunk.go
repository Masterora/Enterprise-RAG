package model

import (
	"encoding/json"
	"time"
)

type DocumentChunk struct {
	ID                   string
	TenantID             string
	DocID                string
	SubjectID            string
	UserID               string
	ChunkIndex           int
	Content              string
	Page                 int
	Section              string
	Metadata             json.RawMessage
	TokenCount           int
	DocumentVersion      int
	ContentHash          string
	EmbeddingProvider    string
	EmbeddingModel       string
	EmbeddingDimension   int
	ChunkStrategyVersion string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type TaskOutbox struct {
	ID            string
	TaskID        string
	Subject       string
	Payload       json.RawMessage
	Headers       map[string]string
	Attempts      int
	NextAttemptAt time.Time
}

type ParseSegment struct {
	Page        int      `json:"page"`
	Section     string   `json:"section"`
	HeadingPath []string `json:"heading_path,omitempty"`
	BlockType   string   `json:"block_type,omitempty"`
	Content     string   `json:"content"`
}

type ChunkMetadata struct {
	HeadingPath []string `json:"heading_path,omitempty"`
	BlockType   string   `json:"block_type,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	SourceType  string   `json:"source_type,omitempty"`
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
	TenantID  string
	DocID     string
	SubjectID string
	Status    string
	PageSize  int
	Offset    int
}
