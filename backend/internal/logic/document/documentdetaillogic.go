// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package document

import (
	"context"
	"strings"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/model"
	documentpresenter "enterprise-rag/backend/internal/presenter/document"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DocumentDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDocumentDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DocumentDetailLogic {
	return &DocumentDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DocumentDetailLogic) DocumentDetail(req *types.DocumentDetailReq) (resp *types.DocumentDetailResp, err error) {
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	document, err := l.svcCtx.DocumentRepo.GetByIDForUser(l.ctx, strings.TrimSpace(req.ID), user.ID)
	if err != nil {
		return nil, err
	}
	chunks, err := l.svcCtx.ChunkRepo.ListByDocument(l.ctx, document.ID)
	if err != nil {
		return nil, err
	}
	tasks, err := l.svcCtx.IndexTaskRepo.ListByDocument(l.ctx, document.ID, user.ID)
	if err != nil {
		return nil, err
	}
	logs, _, err := l.svcCtx.DocumentRepo.ListParseLogs(l.ctx, model.ParseLogListFilter{
		UserID:   user.ID,
		DocID:    document.ID,
		PageSize: 100,
	})
	if err != nil {
		return nil, err
	}

	result := &types.DocumentDetailResp{
		Document: documentpresenter.ToInfo(*document),
		Chunks:   make([]types.DocumentChunkInfo, 0, len(chunks)),
		Tasks:    make([]types.IndexTaskInfo, 0, len(tasks)),
		Logs:     make([]types.ParseLogInfo, 0),
	}
	for _, chunk := range chunks {
		result.Chunks = append(result.Chunks, documentpresenter.ChunkToInfo(chunk))
	}
	for _, item := range tasks {
		result.Tasks = append(result.Tasks, documentpresenter.TaskToInfo(item))
	}
	for _, item := range logs {
		result.Logs = append(result.Logs, documentpresenter.LogToInfo(item))
	}
	return result, nil
}
