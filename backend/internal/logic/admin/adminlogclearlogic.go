// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"strings"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminLogClearLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminLogClearLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminLogClearLogic {
	return &AdminLogClearLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminLogClearLogic) AdminLogClear(req *types.AdminLogClearReq) (resp *types.AdminLogClearResp, err error) {
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	cleared, err := l.svcCtx.DocumentRepo.ClearParseLogsByUser(l.ctx, user.ID, strings.TrimSpace(req.SubjectID))
	if err != nil {
		return nil, err
	}
	return &types.AdminLogClearResp{Cleared: cleared}, nil
}
