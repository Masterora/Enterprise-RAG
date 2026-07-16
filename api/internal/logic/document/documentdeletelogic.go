// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package document

import (
	"context"
	"strings"
	"time"

	"enterprise-rag/api/internal/auth"
	"enterprise-rag/api/internal/model"
	documentpresenter "enterprise-rag/api/internal/presenter/document"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

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
		ID:                   uuid.NewString(),
		TenantID:             user.TenantID,
		DocID:                document.ID,
		SubjectID:            document.SubjectID,
		UserID:               user.ID,
		Filename:             document.Filename,
		TaskType:             model.TaskTypeDelete,
		Status:               model.TaskStatusPending,
		DocumentVersion:      document.DocumentVersion,
		ContentHash:          document.ContentHash,
		EmbeddingProvider:    document.EmbeddingProvider,
		EmbeddingModel:       document.EmbeddingModel,
		EmbeddingDimension:   document.EmbeddingDimension,
		ChunkStrategyVersion: document.ChunkStrategyVersion,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := l.svcCtx.DocumentRepo.CreateDeleteTask(l.ctx, document.ID, user.ID, task); err != nil {
		return nil, err
	}
	return &types.DocumentDeleteResp{Task: documentpresenter.TaskToInfo(*task)}, nil
}
