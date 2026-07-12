// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package chat

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChatSessionUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatSessionUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatSessionUpdateLogic {
	return &ChatSessionUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChatSessionUpdateLogic) ChatSessionUpdate(req *types.ChatSessionUpdateReq) (resp *types.ChatSessionUpdateResp, err error) {
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, errors.New("title is required")
	}
	updated, err := l.svcCtx.ChatRepo.UpdateSession(l.ctx, req.ID, user.ID, title)
	return &types.ChatSessionUpdateResp{Updated: updated}, err
}
