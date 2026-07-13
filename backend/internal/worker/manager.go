package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/infrastructure/observability"
	"enterprise-rag/backend/internal/infrastructure/parser"
	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/service/documentchunk"
	"enterprise-rag/backend/internal/service/taskqueue"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/task"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
)

const (
	maxAutomaticRetries = 3
	automaticRetryDelay = 2 * time.Second
)

type Manager struct {
	svcCtx *svc.ServiceContext
}

func NewManager(svcCtx *svc.ServiceContext) (*Manager, error) {
	if err := validateConfig(svcCtx.Config.Worker); err != nil {
		return nil, err
	}
	return &Manager{
		svcCtx: svcCtx,
	}, nil
}

func validateConfig(value config.WorkerConf) error {
	concurrency := map[string]int{
		model.TaskTypeParse:     value.ParseConcurrency,
		model.TaskTypeChunk:     value.ChunkConcurrency,
		model.TaskTypeEmbedding: value.EmbeddingConcurrency,
		model.TaskTypeDelete:    value.DeleteConcurrency,
	}
	for taskType, value := range concurrency {
		if value < 1 {
			return fmt.Errorf("worker task %q concurrency must be greater than zero", taskType)
		}
	}
	return nil
}

func (m *Manager) Start() error {
	handlers := []struct {
		subject     string
		queue       string
		concurrency int
		handler     nats.MsgHandler
	}{
		{subject: model.TaskTypeParse, queue: "parse-workers", concurrency: m.svcCtx.Config.Worker.ParseConcurrency, handler: m.handleParse},
		{subject: model.TaskTypeChunk, queue: "chunk-workers", concurrency: m.svcCtx.Config.Worker.ChunkConcurrency, handler: m.handleChunk},
		{subject: model.TaskTypeEmbedding, queue: "embedding-workers", concurrency: m.svcCtx.Config.Worker.EmbeddingConcurrency, handler: m.handleEmbedding},
		{subject: model.TaskTypeDelete, queue: "delete-workers", concurrency: m.svcCtx.Config.Worker.DeleteConcurrency, handler: m.handleDelete},
	}
	for _, definition := range handlers {
		if err := m.subscribe(definition.subject, definition.queue, definition.concurrency, definition.handler); err != nil {
			return err
		}
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
	m.run(msg, model.TaskTypeParse, m.parseDocument)
}

func (m *Manager) handleChunk(msg *nats.Msg) {
	m.run(msg, model.TaskTypeChunk, m.chunkDocument)
}

func (m *Manager) handleEmbedding(msg *nats.Msg) {
	m.run(msg, model.TaskTypeEmbedding, m.embedDocument)
}

func (m *Manager) handleDelete(msg *nats.Msg) {
	m.run(msg, model.TaskTypeDelete, m.deleteDocument)
}

func (m *Manager) run(messageEnvelope *nats.Msg, taskType string, fn func(context.Context, *task.Message) error) {
	ctx := taskqueue.ExtractContext(context.Background(), messageEnvelope)
	ctx, taskSpan := observability.StartSpan(ctx, "worker.task",
		attribute.String("worker.task_type", taskType),
	)
	startedAt := time.Now()
	status := "error"
	m.svcCtx.Metrics.WorkerStarted(taskType)
	defer func() {
		m.svcCtx.Metrics.WorkerFinished(taskType)
		m.svcCtx.Metrics.ObserveWorker(taskType, status, time.Since(startedAt))
		observability.EndSpanWithStatus(taskSpan, status)
	}()
	var message task.Message
	if messageEnvelope == nil {
		return
	}
	if err := json.Unmarshal(messageEnvelope.Data, &message); err != nil {
		taskSpan.RecordError(err)
		return
	}
	if message.TaskID == "" || message.DocID == "" {
		return
	}

	if err := m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, message.TaskID, model.TaskStatusRunning, ""); err != nil {
		taskSpan.RecordError(err)
		return
	}
	if err := fn(ctx, &message); err != nil {
		taskSpan.RecordError(err)
		if m.scheduleRetry(ctx, taskType, &message, err) {
			status = "retry"
			return
		}
		documentStatus := model.DocumentStatusFailed
		if taskType == model.TaskTypeDelete {
			documentStatus = model.DocumentStatusDeleteFailed
		}
		_ = m.svcCtx.DocumentRepo.UpdateStatus(ctx, message.DocID, documentStatus, err.Error())
		_ = m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, message.TaskID, model.TaskStatusFailed, err.Error())
		return
	}
	status = "success"
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
		if err := m.svcCtx.DocumentRepo.ResetStatusForRetry(ctx, message.DocID, documentStatus); err != nil {
			_ = m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, message.TaskID, model.TaskStatusFailed, err.Error())
			return false
		}
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
	retryCtx := context.WithoutCancel(ctx)
	go func() {
		time.Sleep(automaticRetryDelay)
		_ = taskqueue.Publish(retryCtx, m.svcCtx.Nats, taskType, message.TaskID, message.DocID, message.ProcessingMode)
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
	if err := m.svcCtx.DocumentRepo.CompleteParse(ctx, doc.ID, plainText, metadata); err != nil {
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
	return taskqueue.Publish(ctx, m.svcCtx.Nats, model.TaskTypeChunk, nextTask.ID, doc.ID, "")
}

func (m *Manager) chunkDocument(ctx context.Context, message *task.Message) error {
	doc, err := m.svcCtx.DocumentRepo.GetByID(ctx, message.DocID)
	if err != nil {
		return err
	}
	if err := m.svcCtx.DocumentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusChunking, ""); err != nil {
		return err
	}

	chunks, err := documentchunk.Build(doc, m.svcCtx.Config.Chunking)
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
	return taskqueue.Publish(ctx, m.svcCtx.Nats, model.TaskTypeEmbedding, nextTask.ID, doc.ID, "")
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

	embeddingStartedAt := time.Now()
	embeddingCtx := m.svcCtx.Metrics.ModelUsageContext(ctx, "embedding", "document_embedding",
		m.svcCtx.Config.Embedding.Provider)
	vectors, err := m.svcCtx.Embedder.Embed(embeddingCtx, texts)
	m.svcCtx.Metrics.ObserveModel("embedding", "document_embedding", m.svcCtx.Config.Embedding.Provider, workerOutcome(err), time.Since(embeddingStartedAt))
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

func workerOutcome(err error) string {
	if err == nil {
		return "success"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "error"
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
	parts := make([]string, 0, 3)
	if section != "" && section != "text" {
		parts = append(parts, section)
	}
	var metadata model.ChunkMetadata
	if len(chunk.Metadata) > 0 && json.Unmarshal(chunk.Metadata, &metadata) == nil {
		if len(metadata.Keywords) > 0 {
			parts = append(parts, strings.Join(metadata.Keywords, " "))
		}
		if metadata.Summary != "" && metadata.Summary != content {
			parts = append(parts, metadata.Summary)
		}
	}
	parts = append(parts, content)
	return strings.Join(parts, "\n")
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
