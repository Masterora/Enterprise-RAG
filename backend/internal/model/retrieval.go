package model

type RetrievalChunk struct {
	ID         string
	DocID      string
	SubjectID  string
	UserID     string
	ChunkIndex int64
	Page       int64
	Section    string
	Content    string
	Score      float64
}
