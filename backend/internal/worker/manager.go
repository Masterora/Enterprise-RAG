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
	"enterprise-rag/backend/internal/service/taskqueue"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/task"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/nats-io/nats.go"
)

const (
	maxAutomaticRetries = 3
	automaticRetryDelay = 2 * time.Second
)

type Manager struct {
	svcCtx *svc.ServiceContext
}

func NewManager(ctx context.Context, svcCtx *svc.ServiceContext) (*Manager, error) {
	if svcCtx.Config.Worker.ParseConcurrency < 1 {
		return nil, errors.New("worker parse concurrency must be greater than zero")
	}
	if svcCtx.Config.Worker.ChunkConcurrency < 1 {
		return nil, errors.New("worker chunk concurrency must be greater than zero")
	}
	if svcCtx.Config.Worker.EmbeddingConcurrency < 1 {
		return nil, errors.New("worker embedding concurrency must be greater than zero")
	}
	if svcCtx.Config.Worker.DeleteConcurrency < 1 {
		return nil, errors.New("worker delete concurrency must be greater than zero")
	}
	return &Manager{
		svcCtx: svcCtx,
	}, nil
}

func (m *Manager) Start() error {
	if err := m.subscribe(
		model.TaskTypeParse,
		"parse-workers",
		m.svcCtx.Config.Worker.ParseConcurrency,
		m.handleParse,
	); err != nil {
		return err
	}
	if err := m.subscribe(
		model.TaskTypeChunk,
		"chunk-workers",
		m.svcCtx.Config.Worker.ChunkConcurrency,
		m.handleChunk,
	); err != nil {
		return err
	}
	if err := m.subscribe(
		model.TaskTypeEmbedding,
		"embedding-workers",
		m.svcCtx.Config.Worker.EmbeddingConcurrency,
		m.handleEmbedding,
	); err != nil {
		return err
	}
	if err := m.subscribe(
		model.TaskTypeDelete,
		"delete-workers",
		m.svcCtx.Config.Worker.DeleteConcurrency,
		m.handleDelete,
	); err != nil {
		return err
	}
	return m.svcCtx.Nats.Flush()
}

func (m *Manager) subscribe(subject, queue string, concurrency int, handler nats.MsgHandler) error {
	for range concurrency {
		if _, err := m.svcCtx.Nats.QueueSubscribe(subject, queue, handler); err != nil {
			return fmt.Errorf("subscribe %s worker: %w", subject, err)
		}
	}
	return nil
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

func (m *Manager) handleDelete(msg *nats.Msg) {
	m.run(msg.Data, model.TaskTypeDelete, m.deleteDocument)
}

func (m *Manager) run(data []byte, taskType string, fn func(context.Context, *task.Message) error) {
	ctx := context.Background()
	var message task.Message
	if err := json.Unmarshal(data, &message); err != nil {
		return
	}
	if message.TaskID == "" || message.DocID == "" {
		return
	}

	if err := m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, message.TaskID, model.TaskStatusRunning, ""); err != nil {
		return
	}
	if err := fn(ctx, &message); err != nil {
		if m.scheduleRetry(ctx, taskType, &message, err) {
			return
		}
		documentStatus := model.DocumentStatusFailed
		if taskType == model.TaskTypeDelete {
			documentStatus = model.DocumentStatusDeleteFailed
		}
		_ = m.svcCtx.DocumentRepo.UpdateStatus(ctx, message.DocID, documentStatus, err.Error())
		_ = m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, message.TaskID, model.TaskStatusFailed, err.Error())
	}
}

func (m *Manager) scheduleRetry(ctx context.Context, taskType string, message *task.Message, taskErr error) bool {
	if !shouldAutoRetry(taskErr) {
		return false
	}
	retriedTask, scheduled, err := m.svcCtx.IndexTaskRepo.ScheduleRetry(ctx, message.TaskID, maxAutomaticRetries)
	if err != nil || !scheduled {
		return false
	}
	if documentStatus, ok := retryPendingDocumentStatus(taskType); ok {
		_ = m.svcCtx.DocumentRepo.UpdateStatus(ctx, message.DocID, documentStatus, "")
	}
	if taskType == model.TaskTypeParse {
		_ = m.svcCtx.DocumentRepo.AddParseLog(ctx, &model.DocumentParseLog{
			ID:        uuid.NewString(),
			DocID:     message.DocID,
			Status:    model.TaskStatusPending,
			Message:   fmt.Sprintf("parse retry scheduled (%d/%d)", retriedTask.RetryCount, maxAutomaticRetries),
			CreatedAt: time.Now(),
		})
	}
	go func() {
		time.Sleep(automaticRetryDelay)
		_ = taskqueue.Publish(m.svcCtx.Nats, taskType, message.TaskID, message.DocID, message.ProcessingMode)
	}()
	return true
}

func retryPendingDocumentStatus(taskType string) (string, bool) {
	switch taskType {
	case model.TaskTypeParse:
		return model.DocumentStatusUploaded, true
	case model.TaskTypeChunk:
		return model.DocumentStatusParsed, true
	case model.TaskTypeEmbedding:
		return model.DocumentStatusChunked, true
	case model.TaskTypeDelete:
		return model.DocumentStatusDeleting, true
	default:
		return "", false
	}
}

func shouldAutoRetry(err error) bool {
	normalized := strings.ToLower(strings.TrimSpace(err.Error()))
	if normalized == "" {
		return false
	}
	switch {
	case strings.Contains(normalized, "invalid api"),
		strings.Contains(normalized, "incorrect api key"),
		strings.Contains(normalized, "unsupported document type"),
		strings.Contains(normalized, "not supported"),
		strings.Contains(normalized, "encryption version"),
		strings.Contains(normalized, "暂不支持"),
		strings.Contains(normalized, "no document chunks found"),
		strings.Contains(normalized, "vector count mismatch"):
		return false
	case strings.Contains(normalized, "timeout"),
		strings.Contains(normalized, "deadline exceeded"),
		strings.Contains(normalized, "tempor"),
		strings.Contains(normalized, "connection reset"),
		strings.Contains(normalized, "connection refused"),
		strings.Contains(normalized, "unexpected eof"),
		strings.Contains(normalized, "broken pipe"),
		strings.Contains(normalized, "try again"):
		return true
	default:
		return false
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

	plainText, metadata, err := parser.Parse(ctx, m.svcCtx.MinIO, bucket, objectName, doc.FileType, parser.ParseOptions{
		ProcessingMode: message.ProcessingMode,
	})
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
	if err := m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, message.TaskID, model.TaskStatusSuccess, ""); err != nil {
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
	return taskqueue.Publish(m.svcCtx.Nats, model.TaskTypeChunk, nextTask.ID, doc.ID, "")
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
	if err := m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, message.TaskID, model.TaskStatusSuccess, ""); err != nil {
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
	return taskqueue.Publish(m.svcCtx.Nats, model.TaskTypeEmbedding, nextTask.ID, doc.ID, "")
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
		texts = append(texts, buildEmbeddingText(chunk))
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
	return m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, message.TaskID, model.TaskStatusSuccess, "")
}

func (m *Manager) deleteDocument(ctx context.Context, message *task.Message) error {
	doc, err := m.svcCtx.DocumentRepo.GetByID(ctx, message.DocID)
	if err != nil {
		return err
	}
	if err := m.svcCtx.MilvusStore.DeleteByDocIDs(ctx, doc.UserID, []string{doc.ID}); err != nil {
		return fmt.Errorf("delete Milvus vectors: %w", err)
	}
	hasResidualVectors, err := m.svcCtx.MilvusStore.HasDocVectors(ctx, doc.UserID, doc.ID)
	if err != nil {
		return fmt.Errorf("verify Milvus vectors: %w", err)
	}
	if hasResidualVectors {
		return errors.New("milvus vectors still exist after delete")
	}
	bucket, objectName, err := parseFileURL(doc.FileURL)
	if err != nil {
		return err
	}
	if err := m.svcCtx.MinIO.RemoveObject(ctx, bucket, objectName, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete MinIO object: %w", err)
	}
	if err := m.svcCtx.DocumentRepo.CompleteDelete(ctx, doc.ID); err != nil {
		return fmt.Errorf("soft delete document: %w", err)
	}
	return m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, message.TaskID, model.TaskStatusSuccess, "")
}

func buildEmbeddingText(chunk model.DocumentChunk) string {
	content := strings.TrimSpace(chunk.Content)
	section := strings.TrimSpace(chunk.Section)
	if section == "" || section == "text" {
		return content
	}
	return section + "\n" + content
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
