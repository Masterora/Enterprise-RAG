package model

type RetrievalChunk struct {
	ID              string
	EvidenceID      string
	TenantID        string
	DocID           string
	SubjectID       string
	UserID          string
	ChunkIndex      int64
	Page            int64
	Section         string
	Content         string
	DocumentVersion int
	ContentHash     string
	Score           float64
}
