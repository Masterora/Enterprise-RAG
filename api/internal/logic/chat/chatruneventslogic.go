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

type ChatRunEventsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatRunEventsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatRunEventsLogic {
	return &ChatRunEventsLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *ChatRunEventsLogic) ChatRunEvents(req *types.ChatRunEventsReq) (*types.ChatRunEventsResp, error) {
	runID := strings.TrimSpace(req.RunID)
	if _, err := uuid.Parse(runID); err != nil {
		return nil, errors.New("invalid run id")
	}
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	events, err := l.svcCtx.RunRepo.ListEvents(l.ctx, runID, user.TenantID, user.ID, req.AfterSequence, req.Limit)
	if err != nil {
		return nil, err
	}
	result := make([]types.ChatRunEventInfo, 0, len(events))
	for _, event := range events {
		payload := make(map[string]any)
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return nil, err
		}
		result = append(result, types.ChatRunEventInfo{
			Sequence: event.Sequence, Type: event.Type, Payload: payload, CreatedAt: event.CreatedAt.Format(timeLayout),
		})
	}
	return &types.ChatRunEventsResp{List: result}, nil
}
