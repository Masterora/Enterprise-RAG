package chat

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/api/internal/auth"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

	"github.com/google/uuid"
)

type ChatRunCancelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatRunCancelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatRunCancelLogic {
	return &ChatRunCancelLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *ChatRunCancelLogic) ChatRunCancel(req *types.ChatRunReq) (*types.ChatRunCancelResp, error) {
	runID := strings.TrimSpace(req.RunID)
	if _, err := uuid.Parse(runID); err != nil {
		return nil, errors.New("invalid run id")
	}
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	requested, err := l.svcCtx.RunRepo.RequestCancel(l.ctx, runID, user.TenantID, user.ID)
	if err != nil {
		return nil, err
	}
	if requested {
		l.svcCtx.RunController.Cancel(runID)
		_ = appendRunEvent(l.ctx, l.svcCtx, user.TenantID, runID, "run.cancel_requested", map[string]any{"run_id": runID})
	}
	return &types.ChatRunCancelResp{Cancelled: requested}, nil
}
