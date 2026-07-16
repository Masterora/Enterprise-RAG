// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package document

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/api/internal/auth"
	documentpresenter "enterprise-rag/api/internal/presenter/document"
	pgrepo "enterprise-rag/api/internal/repository/postgres"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

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
	return &types.IndexTaskRetryResp{Task: documentpresenter.TaskToInfo(*task)}, nil
}
