package model

const (
	DocumentStatusUploaded     = "uploaded"
	DocumentStatusParsing      = "parsing"
	DocumentStatusParsed       = "parsed"
	DocumentStatusChunking     = "chunking"
	DocumentStatusChunked      = "chunked"
	DocumentStatusEmbedding    = "embedding"
	DocumentStatusIndexed      = "indexed"
	DocumentStatusFailed       = "failed"
	DocumentStatusDeleting     = "deleting"
	DocumentStatusDeleteFailed = "delete_failed"
)

const (
	TaskTypeParse     = "document.parse"
	TaskTypeChunk     = "document.chunk"
	TaskTypeEmbedding = "document.embedding"
	TaskTypeDelete    = "document.delete"
)

const (
	TaskStatusPending = "pending"
	TaskStatusRunning = "running"
	TaskStatusSuccess = "success"
	TaskStatusFailed  = "failed"
)
