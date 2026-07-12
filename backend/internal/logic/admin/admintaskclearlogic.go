// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminTaskClearLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminTaskClearLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminTaskClearLogic {
	return &AdminTaskClearLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminTaskClearLogic) AdminTaskClear() (resp *types.AdminTaskClearResp, err error) {
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	cleared, err := l.svcCtx.IndexTaskRepo.ClearTerminalByUser(l.ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &types.AdminTaskClearResp{Cleared: cleared}, nil
}
