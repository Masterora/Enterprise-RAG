package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"enterprise-rag/backend/internal/infrastructure/parser"
	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/task"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type Manager struct {
	svcCtx *svc.ServiceContext
}

func NewManager(ctx context.Context, svcCtx *svc.ServiceContext) (*Manager, error) {
	return &Manager{
		svcCtx: svcCtx,
	}, nil
}

func (m *Manager) Start() error {
	if _, err := m.svcCtx.Nats.QueueSubscribe(model.TaskTypeParse, "parse-workers", m.handleParse); err != nil {
		return err
	}
	if _, err := m.svcCtx.Nats.QueueSubscribe(model.TaskTypeChunk, "chunk-workers", m.handleChunk); err != nil {
		return err
	}
	if _, err := m.svcCtx.Nats.QueueSubscribe(model.TaskTypeEmbedding, "embedding-workers", m.handleEmbedding); err != nil {
		return err
	}
	return m.svcCtx.Nats.Flush()
}

func (m *Manager) handleParse(msg *nats.Msg) {
	m.run(msg.Data, model.TaskTypeParse, m.parseDocument)
}

func (m *Manager) handleChunk(msg *nats.Msg) {
	m.run(msg.Data, model.TaskTypeChunk, m.chunkDocument)
}

func (m *Manager) handleEmbedding(msg *nats.Msg) {
	m.run(msg.Data, model.TaskTypeEmbedding, m.embedDocument)
}

func (m *Manager) run(data []byte, taskType string, fn func(context.Context, *task.Message) error) {
	ctx := context.Background()
	var message task.Message
	if err := json.Unmarshal(data, &message); err != nil {
		return
	}

	if err := m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, message.DocID, taskType, model.TaskStatusRunning, ""); err != nil {
		return
	}
	if err := fn(ctx, &message); err != nil {
		_ = m.svcCtx.DocumentRepo.UpdateStatus(ctx, message.DocID, model.DocumentStatusFailed, err.Error())
		_ = m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, message.DocID, taskType, model.TaskStatusFailed, err.Error())
	}
}

func (m *Manager) parseDocument(ctx context.Context, message *task.Message) error {
	doc, err := m.svcCtx.DocumentRepo.GetByID(ctx, message.DocID)
	if err != nil {
		return err
	}
	if err := m.svcCtx.DocumentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusParsing, ""); err != nil {
		return err
	}

	bucket, objectName, err := parseFileURL(doc.FileURL)
	if err != nil {
		return err
	}

	plainText, metadata, err := parser.Parse(ctx, m.svcCtx.MinIO, bucket, objectName, doc.FileType)
	parseLog := &model.DocumentParseLog{
		ID:        uuid.NewString(),
		DocID:     doc.ID,
		CreatedAt: time.Now(),
	}
	if err != nil {
		parseLog.Status = model.TaskStatusFailed
		parseLog.Error = err.Error()
		_ = m.svcCtx.DocumentRepo.AddParseLog(ctx, parseLog)
		return err
	}

	parseLog.Status = model.TaskStatusSuccess
	parseLog.Message = "document parsed"
	if err := m.svcCtx.DocumentRepo.AddParseLog(ctx, parseLog); err != nil {
		return err
	}
	if err := m.svcCtx.DocumentRepo.UpdateParseResult(ctx, doc.ID, model.DocumentStatusParsed, plainText, metadata, ""); err != nil {
		return err
	}
	if err := m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, doc.ID, model.TaskTypeParse, model.TaskStatusSuccess, ""); err != nil {
		return err
	}

	nextTask := &model.IndexTask{
		ID:        uuid.NewString(),
		DocID:     doc.ID,
		SubjectID: doc.SubjectID,
		UserID:    doc.UserID,
		TaskType:  model.TaskTypeChunk,
		Status:    model.TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := m.svcCtx.IndexTaskRepo.Create(ctx, nextTask); err != nil {
		return err
	}
	return m.publish(model.TaskTypeChunk, doc.ID)
}

func (m *Manager) chunkDocument(ctx context.Context, message *task.Message) error {
	doc, err := m.svcCtx.DocumentRepo.GetByID(ctx, message.DocID)
	if err != nil {
		return err
	}
	if err := m.svcCtx.DocumentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusChunking, ""); err != nil {
		return err
	}

	chunks, err := buildChunks(doc)
	if err != nil {
		return err
	}
	if err := m.svcCtx.ChunkRepo.ReplaceByDocument(ctx, chunks); err != nil {
		return err
	}
	if err := m.svcCtx.DocumentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusChunked, ""); err != nil {
		return err
	}
	if err := m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, doc.ID, model.TaskTypeChunk, model.TaskStatusSuccess, ""); err != nil {
		return err
	}

	nextTask := &model.IndexTask{
		ID:        uuid.NewString(),
		DocID:     doc.ID,
		SubjectID: doc.SubjectID,
		UserID:    doc.UserID,
		TaskType:  model.TaskTypeEmbedding,
		Status:    model.TaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := m.svcCtx.IndexTaskRepo.Create(ctx, nextTask); err != nil {
		return err
	}
	return m.publish(model.TaskTypeEmbedding, doc.ID)
}

func (m *Manager) embedDocument(ctx context.Context, message *task.Message) error {
	doc, err := m.svcCtx.DocumentRepo.GetByID(ctx, message.DocID)
	if err != nil {
		return err
	}
	if err := m.svcCtx.DocumentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusEmbedding, ""); err != nil {
		return err
	}

	chunks, err := m.svcCtx.ChunkRepo.ListByDocument(ctx, doc.ID)
	if err != nil {
		return err
	}
	if len(chunks) == 0 {
		return errors.New("no document chunks found")
	}

	texts := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		texts = append(texts, chunk.Content)
	}

	vectors, err := m.svcCtx.Embedder.Embed(ctx, texts)
	if err != nil {
		return err
	}
	if len(vectors) != len(chunks) {
		return errors.New("embedding vector count mismatch")
	}
	if err := m.svcCtx.MilvusStore.UpsertChunks(ctx, chunks, vectors); err != nil {
		return err
	}
	if err := m.svcCtx.DocumentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusIndexed, ""); err != nil {
		return err
	}
	return m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, doc.ID, model.TaskTypeEmbedding, model.TaskStatusSuccess, "")
}

func (m *Manager) publish(subject, docID string) error {
	payload, err := json.Marshal(task.Message{DocID: docID})
	if err != nil {
		return err
	}
	return m.svcCtx.Nats.Publish(subject, payload)
}

func parseFileURL(fileURL string) (string, string, error) {
	const prefix = "minio://"
	if !strings.HasPrefix(fileURL, prefix) {
		return "", "", errors.New("invalid minio file url")
	}

	path := strings.TrimPrefix(fileURL, prefix)
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", "", errors.New("invalid minio file url")
	}
	return parts[0], parts[1], nil
}

func buildChunks(doc *model.Document) ([]model.DocumentChunk, error) {
	var metadata model.DocumentMetadata
	if len(doc.Metadata) > 0 {
		if err := json.Unmarshal(doc.Metadata, &metadata); err != nil {
			return nil, err
		}
	}

	if len(metadata.Segments) == 0 {
		metadata.Segments = []model.ParseSegment{{Content: doc.PlainText}}
	}

	const chunkSize = 800
	const overlap = 120

	chunks := make([]model.DocumentChunk, 0)
	index := 0
	now := time.Now()
	for _, segment := range metadata.Segments {
		text := []rune(strings.TrimSpace(segment.Content))
		for start := 0; start < len(text); {
			end := start + chunkSize
			if end > len(text) {
				end = len(text)
			}
			content := strings.TrimSpace(string(text[start:end]))
			if content != "" {
				chunks = append(chunks, model.DocumentChunk{
					ID:         uuid.NewString(),
					DocID:      doc.ID,
					SubjectID:  doc.SubjectID,
					UserID:     doc.UserID,
					ChunkIndex: index,
					Content:    content,
					Page:       segment.Page,
					Section:    segment.Section,
					TokenCount: len([]rune(content)),
					CreatedAt:  now,
					UpdatedAt:  now,
				})
				index++
			}
			if end == len(text) {
				break
			}
			start = end - overlap
			if start < 0 {
				start = 0
			}
		}
	}

	if len(chunks) == 0 {
		return nil, fmt.Errorf("document %s has no chunkable content", doc.ID)
	}
	return chunks, nil
}
