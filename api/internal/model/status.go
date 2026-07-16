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

var documentTransitions = map[string][]string{
	DocumentStatusParsing:      {DocumentStatusUploaded},
	DocumentStatusParsed:       {DocumentStatusParsing},
	DocumentStatusChunking:     {DocumentStatusParsed},
	DocumentStatusChunked:      {DocumentStatusChunking},
	DocumentStatusEmbedding:    {DocumentStatusChunked},
	DocumentStatusIndexed:      {DocumentStatusEmbedding},
	DocumentStatusFailed:       {DocumentStatusUploaded, DocumentStatusParsing, DocumentStatusParsed, DocumentStatusChunking, DocumentStatusChunked, DocumentStatusEmbedding},
	DocumentStatusDeleting:     {DocumentStatusUploaded, DocumentStatusParsing, DocumentStatusParsed, DocumentStatusChunking, DocumentStatusChunked, DocumentStatusEmbedding, DocumentStatusIndexed, DocumentStatusFailed, DocumentStatusDeleteFailed},
	DocumentStatusDeleteFailed: {DocumentStatusDeleting},
}

var documentRetryTransitions = map[string][]string{
	DocumentStatusUploaded: {DocumentStatusFailed, DocumentStatusParsing},
	DocumentStatusParsed:   {DocumentStatusFailed, DocumentStatusChunking},
	DocumentStatusChunked:  {DocumentStatusFailed, DocumentStatusEmbedding},
	DocumentStatusDeleting: {DocumentStatusDeleteFailed},
}

func DocumentPreviousStatuses(next string) ([]string, bool) {
	statuses, ok := documentTransitions[next]
	return append([]string(nil), statuses...), ok
}

func DocumentRetryPreviousStatuses(next string) ([]string, bool) {
	statuses, ok := documentRetryTransitions[next]
	return append([]string(nil), statuses...), ok
}

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

var taskTransitions = map[string][]string{
	TaskStatusRunning: {TaskStatusPending},
	TaskStatusSuccess: {TaskStatusRunning},
	TaskStatusFailed:  {TaskStatusPending, TaskStatusRunning},
}

func TaskPreviousStatuses(next string) ([]string, bool) {
	statuses, ok := taskTransitions[next]
	return append([]string(nil), statuses...), ok
}
