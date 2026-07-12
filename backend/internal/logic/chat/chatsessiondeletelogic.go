// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package chat

import (
	"context"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChatSessionDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatSessionDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatSessionDeleteLogic {
	return &ChatSessionDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChatSessionDeleteLogic) ChatSessionDelete(req *types.ChatSessionDeleteReq) (resp *types.ChatSessionDeleteResp, err error) {
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	deleted, err := l.svcCtx.ChatRepo.DeleteSession(l.ctx, req.ID, user.ID)
	return &types.ChatSessionDeleteResp{Deleted: deleted}, err
}
