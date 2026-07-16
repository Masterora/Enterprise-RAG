// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package chat

import (
	"context"

	"enterprise-rag/api/internal/auth"
	chatpresenter "enterprise-rag/api/internal/presenter/chat"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ChatSessionListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatSessionListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatSessionListLogic {
	return &ChatSessionListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChatSessionListLogic) ChatSessionList() (resp *types.ChatSessionListResp, err error) {
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	sessions, err := l.svcCtx.ChatRepo.ListSessions(l.ctx, user.ID)
	if err != nil {
		return nil, err
	}
	list := make([]types.ChatSessionInfo, 0, len(sessions))
	for _, session := range sessions {
		list = append(list, chatpresenter.MapSession(session))
	}
	return &types.ChatSessionListResp{List: list}, nil
}
