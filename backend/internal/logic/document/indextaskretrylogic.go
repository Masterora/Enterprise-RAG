// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package document

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/model"
	documentpresenter "enterprise-rag/backend/internal/presenter/document"
	pgrepo "enterprise-rag/backend/internal/repository/postgres"
	"enterprise-rag/backend/internal/service/taskqueue"
	"enterprise-rag/backend/internal/svc"
	taskmsg "enterprise-rag/backend/internal/task"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type IndexTaskRetryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewIndexTaskRetryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IndexTaskRetryLogic {
	return &IndexTaskRetryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IndexTaskRetryLogic) IndexTaskRetry(req *types.IndexTaskRetryReq) (resp *types.IndexTaskRetryResp, err error) {
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	task, err := l.svcCtx.IndexTaskRepo.Retry(l.ctx, strings.TrimSpace(req.ID), user.ID)
	if err != nil {
		if errors.Is(err, pgrepo.ErrIndexTaskNotRetryable) {
			return nil, errors.New("task is no longer retryable")
		}
		return nil, err
	}
	documentStatus, ok := retryDocumentStatus(task.TaskType)
	if !ok {
		return nil, errors.New("unsupported task type")
	}
	if err := l.svcCtx.DocumentRepo.UpdateStatus(l.ctx, task.DocID, documentStatus, ""); err != nil {
		_ = l.svcCtx.IndexTaskRepo.UpdateStatus(l.ctx, task.ID, model.TaskStatusFailed, err.Error())
		return nil, err
	}
	processingMode := ""
	if task.TaskType == model.TaskTypeParse {
		var metadata taskmsg.ParseTaskMetadata
		if err := json.Unmarshal(task.Metadata, &metadata); err == nil {
			processingMode = taskmsg.NormalizeProcessingMode(metadata.ProcessingMode)
		}
	}
	if err := taskqueue.Publish(l.svcCtx.Nats, task.TaskType, task.ID, task.DocID, processingMode); err != nil {
		_ = l.svcCtx.IndexTaskRepo.UpdateStatus(l.ctx, task.ID, model.TaskStatusFailed, err.Error())
		failedStatus := model.DocumentStatusFailed
		if task.TaskType == model.TaskTypeDelete {
			failedStatus = model.DocumentStatusDeleteFailed
		}
		_ = l.svcCtx.DocumentRepo.UpdateStatus(l.ctx, task.DocID, failedStatus, err.Error())
		return nil, err
	}

	return &types.IndexTaskRetryResp{Task: documentpresenter.TaskToInfo(*task)}, nil
}

func retryDocumentStatus(taskType string) (string, bool) {
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
