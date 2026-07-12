// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package chat

import (
	"context"
	"errors"
	"strings"
	"time"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/model"
	chatpresenter "enterprise-rag/backend/internal/presenter/chat"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type ChatSessionCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatSessionCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatSessionCreateLogic {
	return &ChatSessionCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChatSessionCreateLogic) ChatSessionCreate(req *types.ChatSessionCreateReq) (resp *types.ChatSessionCreateResp, err error) {
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(req.ID); err != nil {
		return nil, errors.New("invalid session id")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, errors.New("title is required")
	}
	if req.SubjectID != "" {
		exists, err := l.svcCtx.SubjectRepo.ExistsAccessible(l.ctx, req.SubjectID, user.ID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, errors.New("knowledge base not found")
		}
	}
	now := time.Now()
	session := model.ChatSession{
		ID: req.ID, UserID: user.ID, SubjectID: req.SubjectID, Title: title,
		LLMProvider: req.LlmProvider, LLMModel: req.LlmModel, CreatedAt: now, UpdatedAt: now,
	}
	if err := l.svcCtx.ChatRepo.CreateSession(l.ctx, &session); err != nil {
		return nil, err
	}
	return &types.ChatSessionCreateResp{Session: chatpresenter.MapSession(session)}, nil
}
