// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package chat

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/api/internal/auth"
	agentinfra "enterprise-rag/api/internal/infrastructure/agent"
	"enterprise-rag/api/internal/service/conversation"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type ChatAskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatAskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatAskLogic {
	return &ChatAskLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ChatAskLogic) ChatAsk(req *types.ChatAskReq) (*types.ChatAskResp, error) {
	if strings.TrimSpace(req.SubjectID) == "" || strings.TrimSpace(req.Query) == "" {
		return nil, errors.New("subject_id and query are required")
	}
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	runID := uuid.NewString()
	runCtx, cleanup, err := beginRun(l.ctx, l.svcCtx, user, req, runID, true)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	output, err := l.svcCtx.Agent.Invoke(runCtx, agentRequest(runID, user.TenantID, user.ID, req, l.svcCtx.Config.Retrieval.TopK))
	if err != nil {
		failRun(l.ctx, l.svcCtx, user, runID, effectiveRunError(runCtx, err))
		logx.WithContext(l.ctx).Errorf("agent chat failed: user_id=%s subject_id=%s err=%v", user.ID, req.SubjectID, err)
		return nil, err
	}
	if err := conversation.PersistTurn(runCtx, l.svcCtx, user.ID, req, output.Answer, output.Chunks,
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

func agentRequest(runID, tenantID, userID string, req *types.ChatAskReq, defaultTopK int) agentinfra.Request {
	topK := req.TopK
	if topK <= 0 {
		topK = defaultTopK
	}
	expectedDocIDs := append([]string{}, req.ExpectedDocIDs...)
	expectedChunkIDs := append([]string{}, req.ExpectedChunkIDs...)
	return agentinfra.Request{
		RunID: runID, TenantID: tenantID,
		SessionID: req.SessionID, MessageID: req.MessageID, UserID: userID, SubjectID: req.SubjectID,
		Question: req.Query, TopK: topK, LLMProvider: req.LlmProvider, LLMModel: req.LlmModel,
		WebSearch: req.WebSearch, ExpectedDocIDs: expectedDocIDs, ExpectedChunkIDs: expectedChunkIDs,
		ExpectedRoute: req.ExpectedRoute, ExpectedOutcome: req.ExpectedOutcome,
	}
}
