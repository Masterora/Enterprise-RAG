// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package chat

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/service/agent"
	"enterprise-rag/backend/internal/service/chatflow"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

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
	runner, err := agent.NewRunner(l.svcCtx)
	if err != nil {
		return nil, err
	}
	output, err := runner.Run(l.ctx, agentInput(user.ID, req), agent.Callbacks{}, false)
	if err != nil {
		logx.WithContext(l.ctx).Errorf("agent chat failed: user_id=%s subject_id=%s failure_kind=%s err=%v",
			user.ID, req.SubjectID, chatflow.GenerationFailureKind(err), err)
		return nil, err
	}
	if err := chatflow.PersistTurn(l.ctx, l.svcCtx, user.ID, req, output.Answer, output.Chunks,
		output.ExternalLinks, output.Metrics, output.Steps); err != nil {
		return nil, err
	}
	return &types.ChatAskResp{
		Answer: output.Answer, Chunks: output.Chunks, ExternalLinks: output.ExternalLinks,
		Metrics: output.Metrics, AgentSteps: output.Steps,
	}, nil
}

func agentInput(userID string, req *types.ChatAskReq) agent.Input {
	return agent.Input{
		SessionID: req.SessionID, MessageID: req.MessageID, UserID: userID, SubjectID: req.SubjectID,
		Question: req.Query, TopK: req.TopK, LLMProvider: req.LlmProvider, LLMModel: req.LlmModel,
		WebSearch: req.WebSearch, ExpectedDocIDs: req.ExpectedDocIDs, ExpectedChunkIDs: req.ExpectedChunkIDs,
		ExpectedRoute: req.ExpectedRoute, ExpectedOutcome: req.ExpectedOutcome,
	}
}
