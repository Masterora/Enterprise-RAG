package document

import (
	"context"

	"enterprise-rag/api/internal/auth"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DocumentClearFailedLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDocumentClearFailedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DocumentClearFailedLogic {
	return &DocumentClearFailedLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DocumentClearFailedLogic) DocumentClearFailed() (*types.DocumentClearFailedResp, error) {
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}

	docIDs, err := l.svcCtx.DocumentRepo.ListActiveFailedDocIDsByUser(l.ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if len(docIDs) > 0 {
		if err := l.svcCtx.MilvusStore.DeleteByDocIDs(l.ctx, user.ID, docIDs); err != nil {
			logx.WithContext(l.ctx).Errorf("clear failed documents milvus delete failed: user_id=%s doc_count=%d err=%v", user.ID, len(docIDs), err)
			return nil, err
		}
	}

	deleted, err := l.svcCtx.DocumentRepo.ClearFailedByUser(l.ctx, user.ID)
	if err != nil {
		return nil, err
	}
	logx.WithContext(l.ctx).Infof("clear failed documents finished: user_id=%s doc_count=%d", user.ID, deleted)

	return &types.DocumentClearFailedResp{Deleted: deleted}, nil
}
