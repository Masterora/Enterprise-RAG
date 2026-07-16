package chat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"enterprise-rag/api/internal/auth"
	"enterprise-rag/api/internal/service/conversation"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

	"github.com/google/uuid"
)

type ChatRunResumeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatRunResumeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatRunResumeLogic {
	return &ChatRunResumeLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *ChatRunResumeLogic) ChatRunResume(input *types.ChatRunReq) (*types.ChatAskResp, error) {
	runID := strings.TrimSpace(input.RunID)
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
	var req types.ChatAskReq
	if err := json.Unmarshal(run.Request, &req); err != nil {
		return nil, err
	}
	if err := l.svcCtx.RunRepo.ResetForResume(l.ctx, runID, user.TenantID, user.ID); err != nil {
		return nil, err
	}
	runCtx, cleanup, err := beginRun(l.ctx, l.svcCtx, user, &req, runID, false)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	output, err := l.svcCtx.Agent.Invoke(runCtx, agentRequest(runID, user.TenantID, user.ID, &req, l.svcCtx.Config.Retrieval.TopK))
	if err != nil {
		failRun(l.ctx, l.svcCtx, user, runID, effectiveRunError(runCtx, err))
		return nil, err
	}
	if err := conversation.PersistTurn(runCtx, l.svcCtx, user.ID, &req, output.Answer, output.Chunks,
		output.ExternalLinks, output.Metrics, output.AgentSteps); err != nil {
		failRun(l.ctx, l.svcCtx, user, runID, effectiveRunError(runCtx, err))
		return nil, err
	}
	if err := completeRun(l.ctx, l.svcCtx, user, runID, output); err != nil {
		failRun(l.ctx, l.svcCtx, user, runID, effectiveRunError(runCtx, err))
		return nil, err
	}
	return &types.ChatAskResp{
		RunID: runID, Answer: output.Answer, Chunks: output.Chunks, ExternalLinks: output.ExternalLinks,
		Metrics: output.Metrics, AgentSteps: output.AgentSteps,
	}, nil
}
