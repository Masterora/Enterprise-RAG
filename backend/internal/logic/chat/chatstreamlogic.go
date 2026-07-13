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

type StreamCallbacks struct {
	OnStatus     func(string) error
	OnAgentStep  func(types.AgentStep) error
	OnSources    func([]types.RetrievalChunk) error
	OnWebSources func([]types.ExternalLink) error
	OnMetrics    func(types.RetrievalMetrics) error
	OnDelta      func(string) error
	OnDone       func(string) error
}

type ChatStreamLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatStreamLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatStreamLogic {
	return &ChatStreamLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ChatStreamLogic) ChatStream(req *types.ChatAskReq, callbacks StreamCallbacks) error {
	if strings.TrimSpace(req.SubjectID) == "" || strings.TrimSpace(req.Query) == "" {
		return errors.New("subject_id and query are required")
	}
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return err
	}
	runner, err := agent.NewRunner(l.svcCtx)
	if err != nil {
		return err
	}
	output, err := runner.Run(l.ctx, agentInput(user.ID, req), agent.Callbacks{
		OnStatus: callbacks.OnStatus, OnStep: callbacks.OnAgentStep, OnSources: callbacks.OnSources,
		OnWebSources: callbacks.OnWebSources, OnMetrics: callbacks.OnMetrics, OnDelta: callbacks.OnDelta,
	}, true)
	if err != nil {
		logx.WithContext(l.ctx).Errorf("agent stream failed: user_id=%s subject_id=%s failure_kind=%s err=%v",
			user.ID, req.SubjectID, chatflow.GenerationFailureKind(err), err)
		return err
	}
	if err := chatflow.PersistTurn(l.ctx, l.svcCtx, user.ID, req, output.Answer, output.Chunks,
		output.ExternalLinks, output.Metrics, output.Steps); err != nil {
		return err
	}
	if callbacks.OnDone != nil {
		return callbacks.OnDone(output.Answer)
	}
	return nil
}
