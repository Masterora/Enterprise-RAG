// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package document

import (
	"context"
	"strings"
	"time"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/model"
	documentpresenter "enterprise-rag/backend/internal/presenter/document"
	"enterprise-rag/backend/internal/service/taskqueue"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type DocumentDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDocumentDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DocumentDeleteLogic {
	return &DocumentDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DocumentDeleteLogic) DocumentDelete(req *types.DocumentDeleteReq) (resp *types.DocumentDeleteResp, err error) {
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	document, err := l.svcCtx.DocumentRepo.GetByIDForUser(l.ctx, strings.TrimSpace(req.ID), user.ID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	task := &model.IndexTask{
		ID:        uuid.NewString(),
		DocID:     document.ID,
		SubjectID: document.SubjectID,
		UserID:    user.ID,
		Filename:  document.Filename,
		TaskType:  model.TaskTypeDelete,
		Status:    model.TaskStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := l.svcCtx.DocumentRepo.CreateDeleteTask(l.ctx, document.ID, user.ID, task); err != nil {
		return nil, err
	}
	if err := taskqueue.Publish(l.ctx, l.svcCtx.Nats, task.TaskType, task.ID, task.DocID, ""); err != nil {
		_ = l.svcCtx.IndexTaskRepo.UpdateStatus(l.ctx, task.ID, model.TaskStatusFailed, err.Error())
		_ = l.svcCtx.DocumentRepo.UpdateStatus(l.ctx, document.ID, model.DocumentStatusDeleteFailed, err.Error())
		return nil, err
	}

	return &types.DocumentDeleteResp{Task: documentpresenter.TaskToInfo(*task)}, nil
}
