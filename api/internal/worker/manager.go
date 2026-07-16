package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"enterprise-rag/api/internal/config"
	"enterprise-rag/api/internal/infrastructure/embedding"
	"enterprise-rag/api/internal/infrastructure/observability"
	"enterprise-rag/api/internal/infrastructure/parser"
	"enterprise-rag/api/internal/model"
	"enterprise-rag/api/internal/service/documentchunk"
	"enterprise-rag/api/internal/service/taskqueue"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/task"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
)

const (
	maxAutomaticRetries = 3
	automaticRetryDelay = 2 * time.Second
)

type Manager struct {
	svcCtx        *svc.ServiceContext
	ctx           context.Context
	cancel        context.CancelFunc
	lifecycleMu   sync.Mutex
	closing       bool
	subscriptions []*nats.Subscription
	inFlight      sync.WaitGroup
	closeDone     chan struct{}
	closeErr      error
	outboxStop    chan struct{}
	outboxDone    chan struct{}
	outboxStarted bool
}

func NewManager(svcCtx *svc.ServiceContext) (*Manager, error) {
	if err := validateConfig(svcCtx.Config.Worker); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		svcCtx:     svcCtx,
		ctx:        ctx,
		cancel:     cancel,
		closeDone:  make(chan struct{}),
		outboxStop: make(chan struct{}),
		outboxDone: make(chan struct{}),
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
	if value.TaskTimeoutSeconds < 1 {
		return errors.New("worker task timeout must be greater than zero")
	}
	if value.ShutdownTimeoutSeconds < 1 {
		return errors.New("worker shutdown timeout must be greater than zero")
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
			return errors.Join(err, m.unsubscribeAll())
		}
	}
	m.outboxStarted = true
	go m.publishOutbox()
	return m.svcCtx.Nats.Flush()
}

func (m *Manager) publishOutbox() {
	defer close(m.outboxDone)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-m.outboxStop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			events, err := m.svcCtx.OutboxRepo.ClaimBatch(ctx, 50)
			if err == nil {
				for _, event := range events {
					if publishErr := taskqueue.PublishPayloadWithHeaders(ctx, m.svcCtx.JetStream, event.Subject, event.TaskID, event.Payload, event.Headers); publishErr != nil {
						_ = m.svcCtx.OutboxRepo.MarkFailed(ctx, event.ID, publishErr.Error())
						continue
					}
					_ = m.svcCtx.OutboxRepo.MarkPublished(ctx, event.ID)
				}
			}
			cancel()
		}
	}
}

func (m *Manager) subscribe(subject, queue string, concurrency int, handler nats.MsgHandler) error {
	for range concurrency {
		subscription, err := m.svcCtx.JetStream.QueueSubscribe(
			subject,
			queue,
			m.track(handler),
			nats.BindStream(taskqueue.StreamName),
			nats.Durable(queue),
			nats.ManualAck(),
			nats.AckExplicit(),
			nats.DeliverAll(),
			nats.AckWait(time.Duration(m.svcCtx.Config.Worker.TaskTimeoutSeconds+30)*time.Second),
			nats.MaxDeliver(maxAutomaticRetries+2),
		)
		if err != nil {
			return fmt.Errorf("subscribe %s worker: %w", subject, err)
		}
		m.subscriptions = append(m.subscriptions, subscription)
	}
	return nil
}

func (m *Manager) track(handler nats.MsgHandler) nats.MsgHandler {
	return func(message *nats.Msg) {
		m.lifecycleMu.Lock()
		if m.closing {
			m.lifecycleMu.Unlock()
			_ = message.NakWithDelay(automaticRetryDelay)
			return
		}
		m.inFlight.Add(1)
		m.lifecycleMu.Unlock()
		defer m.inFlight.Done()
		handler(message)
	}
}

func (m *Manager) Close(ctx context.Context) error {
	m.lifecycleMu.Lock()
	if !m.closing {
		m.closing = true
		m.closeErr = m.unsubscribeAllLocked()
		if m.outboxStarted {
			close(m.outboxStop)
		}
		go func() {
			if m.outboxStarted {
				<-m.outboxDone
			}
			m.inFlight.Wait()
			m.cancel()
			close(m.closeDone)
		}()
	}
	closeErr := m.closeErr
	closeDone := m.closeDone
	m.lifecycleMu.Unlock()

	select {
	case <-closeDone:
		return closeErr
	case <-ctx.Done():
		m.cancel()
		return errors.Join(closeErr, fmt.Errorf("wait for workers to stop: %w", ctx.Err()))
	}
}

func (m *Manager) unsubscribeAll() error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	return m.unsubscribeAllLocked()
}

func (m *Manager) unsubscribeAllLocked() error {
	var result error
	for _, subscription := range m.subscriptions {
		if err := subscription.Unsubscribe(); err != nil && !errors.Is(err, nats.ErrBadSubscription) {
			result = errors.Join(result, fmt.Errorf("unsubscribe worker: %w", err))
		}
	}
	m.subscriptions = nil
	return result
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
	if messageEnvelope == nil {
		return
	}
	ctx := taskqueue.ExtractContext(m.ctx, messageEnvelope)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(m.svcCtx.Config.Worker.TaskTimeoutSeconds)*time.Second)
	defer cancel()
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
	if err := json.Unmarshal(messageEnvelope.Data, &message); err != nil {
		taskSpan.RecordError(err)
		status = "invalid"
		if dlqErr := taskqueue.PublishDeadLetter(ctx, m.svcCtx.JetStream, taskType, "", messageEnvelope.Data, err); dlqErr != nil {
			_ = messageEnvelope.NakWithDelay(automaticRetryDelay)
			return
		}
		_ = messageEnvelope.Term()
		return
	}
	if message.TaskID == "" || message.DocID == "" {
		status = "invalid"
		invalidErr := errors.New("task_id and doc_id are required")
		if dlqErr := taskqueue.PublishDeadLetter(ctx, m.svcCtx.JetStream, taskType, message.TaskID, messageEnvelope.Data, invalidErr); dlqErr != nil {
			_ = messageEnvelope.NakWithDelay(automaticRetryDelay)
			return
		}
		_ = messageEnvelope.Term()
		return
	}

	claimed, err := m.svcCtx.IndexTaskRepo.Claim(
		ctx,
		message.TaskID,
		time.Duration(m.svcCtx.Config.Worker.TaskTimeoutSeconds)*time.Second,
	)
	if err != nil {
		taskSpan.RecordError(err)
		status = "retry"
		_ = messageEnvelope.NakWithDelay(automaticRetryDelay)
		return
	}
	if !claimed {
		existing, getErr := m.svcCtx.IndexTaskRepo.GetByID(ctx, message.TaskID)
		if getErr == nil && (existing.Status == model.TaskStatusSuccess || existing.Status == model.TaskStatusFailed) {
			status = "duplicate"
			_ = messageEnvelope.AckSync()
			return
		}
		status = "retry"
		_ = messageEnvelope.NakWithDelay(time.Duration(m.svcCtx.Config.Worker.TaskTimeoutSeconds) * time.Second)
		return
	}
	if err := fn(ctx, &message); err != nil {
		taskSpan.RecordError(err)
		recoveryCtx, recoveryCancel := context.WithTimeout(m.ctx, 10*time.Second)
		defer recoveryCancel()
		if m.scheduleRetry(recoveryCtx, taskType, &message, err) {
			status = "retry"
			_ = messageEnvelope.NakWithDelay(automaticRetryDelay)
			return
		}
		if persistErr := m.persistFailure(recoveryCtx, taskType, &message, err); persistErr != nil {
			taskSpan.RecordError(persistErr)
			status = "retry"
			_ = messageEnvelope.NakWithDelay(automaticRetryDelay)
			return
		}
		if dlqErr := taskqueue.PublishDeadLetter(recoveryCtx, m.svcCtx.JetStream, taskType, message.TaskID, messageEnvelope.Data, err); dlqErr != nil {
			taskSpan.RecordError(dlqErr)
			status = "retry"
			_ = messageEnvelope.NakWithDelay(automaticRetryDelay)
			return
		}
		_ = messageEnvelope.AckSync()
		return
	}
	completionCtx, completionCancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer completionCancel()
	if err := m.svcCtx.IndexTaskRepo.UpdateStatus(completionCtx, message.TaskID, model.TaskStatusSuccess, ""); err != nil {
		taskSpan.RecordError(err)
		status = "retry"
		_ = messageEnvelope.NakWithDelay(automaticRetryDelay)
		return
	}
	if err := messageEnvelope.AckSync(); err != nil {
		taskSpan.RecordError(err)
		return
	}
	status = "success"
}

func (m *Manager) persistFailure(ctx context.Context, taskType string, message *task.Message, taskErr error) error {
	documentStatus := model.DocumentStatusFailed
	if taskType == model.TaskTypeDelete {
		documentStatus = model.DocumentStatusDeleteFailed
	}
	if err := m.svcCtx.DocumentRepo.UpdateStatus(ctx, message.DocID, documentStatus, taskErr.Error()); err != nil {
		return fmt.Errorf("persist failed document state: %w", err)
	}
	if err := m.svcCtx.IndexTaskRepo.UpdateStatus(ctx, message.TaskID, model.TaskStatusFailed, taskErr.Error()); err != nil {
		return fmt.Errorf("persist failed task state: %w", err)
	}
	return nil
}

func (m *Manager) scheduleRetry(ctx context.Context, taskType string, message *task.Message, taskErr error) bool {
	if !shouldAutoRetry(taskErr) {
		return false
	}
	retriedTask, scheduled, err := m.svcCtx.IndexTaskRepo.ScheduleRetry(ctx, message.TaskID, maxAutomaticRetries)
	if err != nil || !scheduled {
		return false
	}
	if documentStatus, ok := retryPendingDocumentStatus(taskType); ok && !errors.Is(taskErr, taskqueue.ErrPublish) {
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
	default:
		return "", false
	}
}

func shouldAutoRetry(err error) bool {
	if errors.Is(err, taskqueue.ErrPublish) {
		return true
	}
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
	if documentAtOrBeyond(doc.Status, model.DocumentStatusParsed) {
		return m.enqueueNext(ctx, message, doc, model.TaskTypeChunk)
	}
	if doc.Status != model.DocumentStatusParsing {
		if err := m.svcCtx.DocumentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusParsing, ""); err != nil {
			return err
		}
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
	return m.enqueueNext(ctx, message, doc, model.TaskTypeChunk)
}

func (m *Manager) chunkDocument(ctx context.Context, message *task.Message) error {
	doc, err := m.svcCtx.DocumentRepo.GetByID(ctx, message.DocID)
	if err != nil {
		return err
	}
	if documentAtOrBeyond(doc.Status, model.DocumentStatusChunked) {
		return m.enqueueNext(ctx, message, doc, model.TaskTypeEmbedding)
	}
	if doc.Status != model.DocumentStatusChunking {
		if err := m.svcCtx.DocumentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusChunking, ""); err != nil {
			return err
		}
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
	return m.enqueueNext(ctx, message, doc, model.TaskTypeEmbedding)
}

func (m *Manager) embedDocument(ctx context.Context, message *task.Message) error {
	doc, err := m.svcCtx.DocumentRepo.GetByID(ctx, message.DocID)
	if err != nil {
		return err
	}
	if documentAtOrBeyond(doc.Status, model.DocumentStatusIndexed) {
		return nil
	}
	if doc.Status != model.DocumentStatusEmbedding {
		if err := m.svcCtx.DocumentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusEmbedding, ""); err != nil {
			return err
		}
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
		embedding.ProviderName(m.svcCtx.Config.Embedding))
	vectors, err := m.svcCtx.Embedder.Embed(embeddingCtx, doc.TenantID, texts)
	m.svcCtx.Metrics.ObserveModel("embedding", "document_embedding", embedding.ProviderName(m.svcCtx.Config.Embedding), workerOutcome(err), time.Since(embeddingStartedAt))
	if err != nil {
		return err
	}
	if len(vectors) != len(chunks) {
		return errors.New("embedding vector count mismatch")
	}
	if err := m.svcCtx.MilvusStore.UpsertChunks(ctx, chunks, vectors); err != nil {
		return err
	}
	return m.svcCtx.DocumentRepo.UpdateStatus(ctx, doc.ID, model.DocumentStatusIndexed, "")
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
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
	return nil
}

func (m *Manager) enqueueNext(ctx context.Context, current *task.Message, doc *model.Document, nextType string) error {
	nextTask := &model.IndexTask{
		ID:                   deterministicID(current.TaskID, nextType),
		TenantID:             doc.TenantID,
		DocID:                doc.ID,
		SubjectID:            doc.SubjectID,
		UserID:               doc.UserID,
		TaskType:             nextType,
		Status:               model.TaskStatusPending,
		DocumentVersion:      doc.DocumentVersion,
		ContentHash:          doc.ContentHash,
		EmbeddingProvider:    doc.EmbeddingProvider,
		EmbeddingModel:       doc.EmbeddingModel,
		EmbeddingDimension:   doc.EmbeddingDimension,
		ChunkStrategyVersion: doc.ChunkStrategyVersion,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
	if err := m.svcCtx.IndexTaskRepo.Create(ctx, nextTask); err != nil {
		return err
	}
	return nil
}

func deterministicID(parentID, purpose string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(parentID+":"+purpose)).String()
}

func documentAtOrBeyond(current, target string) bool {
	rank := map[string]int{
		model.DocumentStatusUploaded:  0,
		model.DocumentStatusParsing:   1,
		model.DocumentStatusParsed:    2,
		model.DocumentStatusChunking:  3,
		model.DocumentStatusChunked:   4,
		model.DocumentStatusEmbedding: 5,
		model.DocumentStatusIndexed:   6,
	}
	currentRank, currentOK := rank[current]
	targetRank, targetOK := rank[target]
	return currentOK && targetOK && currentRank >= targetRank
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
