package model

const (
	DocumentStatusUploaded  = "uploaded"
	DocumentStatusParsing   = "parsing"
	DocumentStatusParsed    = "parsed"
	DocumentStatusChunking  = "chunking"
	DocumentStatusChunked   = "chunked"
	DocumentStatusEmbedding = "embedding"
	DocumentStatusIndexed   = "indexed"
	DocumentStatusFailed    = "failed"
)

const (
	TaskTypeParse     = "document.parse"
	TaskTypeChunk     = "document.chunk"
	TaskTypeEmbedding = "document.embedding"
)

const (
	TaskStatusPending = "pending"
	TaskStatusRunning = "running"
	TaskStatusSuccess = "success"
	TaskStatusFailed  = "failed"
)
