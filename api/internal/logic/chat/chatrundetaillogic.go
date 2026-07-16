package chat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"enterprise-rag/api/internal/auth"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

	"github.com/google/uuid"
)

type ChatRunDetailLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatRunDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatRunDetailLogic {
	return &ChatRunDetailLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *ChatRunDetailLogic) ChatRunDetail(req *types.ChatRunReq) (*types.ChatRunDetailResp, error) {
	runID := strings.TrimSpace(req.RunID)
	if _, err := uuid.Parse(runID); err != nil {
		return nil, errors.New("invalid run id")
	}
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	run, err := l.svcCtx.RunRepo.GetForUser(l.ctx, runID, user.TenantID, user.ID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]any)
	if len(run.Result) > 0 {
		if err := json.Unmarshal(run.Result, &result); err != nil {
			return nil, err
		}
	}
	return &types.ChatRunDetailResp{Run: mapRunInfo(run), Result: result}, nil
}
