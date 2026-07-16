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

type StreamCallbacks struct {
	OnRunCreated func(string) error
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
	runID := uuid.NewString()
	runCtx, cleanup, err := beginRun(l.ctx, l.svcCtx, user, req, runID, true)
	if err != nil {
		return err
	}
	defer cleanup()
	if callbacks.OnRunCreated != nil {
		if err := callbacks.OnRunCreated(runID); err != nil {
			failRun(l.ctx, l.svcCtx, user, runID, err)
			return err
		}
	}
	persist := func(eventType string, payload any) error {
		return appendRunEvent(runCtx, l.svcCtx, user.TenantID, runID, eventType, payload)
	}
	output, err := l.svcCtx.Agent.Stream(runCtx, agentRequest(runID, user.TenantID, user.ID, req, l.svcCtx.Config.Retrieval.TopK), agentinfra.Callbacks{
		OnStatus: func(value string) error {
			if err := persist("status", map[string]string{"message": value}); err != nil {
				return err
			}
			return callStreamCallback(callbacks.OnStatus, value)
		},
		OnAgentStep: func(value types.AgentStep) error {
			if err := persist("agent_step", value); err != nil {
				return err
			}
			return callStreamCallback(callbacks.OnAgentStep, value)
		},
		OnSources: func(value []types.RetrievalChunk) error {
			if err := persist("sources", map[string]any{"chunks": value}); err != nil {
				return err
			}
			return callStreamCallback(callbacks.OnSources, value)
		},
		OnWebSources: func(value []types.ExternalLink) error {
			if err := persist("web_sources", map[string]any{"links": value}); err != nil {
				return err
			}
			return callStreamCallback(callbacks.OnWebSources, value)
		},
		OnMetrics: func(value types.RetrievalMetrics) error {
			if err := persist("metrics", value); err != nil {
				return err
			}
			return callStreamCallback(callbacks.OnMetrics, value)
		},
		OnDelta: func(value string) error {
			if err := persist("delta", map[string]string{"content": value}); err != nil {
				return err
			}
			return callStreamCallback(callbacks.OnDelta, value)
		},
	})
	if err != nil {
		failRun(l.ctx, l.svcCtx, user, runID, effectiveRunError(runCtx, err))
		logx.WithContext(l.ctx).Errorf("agent stream failed: user_id=%s subject_id=%s err=%v", user.ID, req.SubjectID, err)
		return err
	}
	if err := conversation.PersistTurn(runCtx, l.svcCtx, user.ID, req, output.Answer, output.Chunks,
		output.ExternalLinks, output.Metrics, output.AgentSteps); err != nil {
		failRun(l.ctx, l.svcCtx, user, runID, effectiveRunError(runCtx, err))
		return err
	}
	if err := completeRun(l.ctx, l.svcCtx, user, runID, output); err != nil {
		failRun(l.ctx, l.svcCtx, user, runID, effectiveRunError(runCtx, err))
		return err
	}
	if callbacks.OnDone != nil {
		return callbacks.OnDone(output.Answer)
	}
	return nil
}

func callStreamCallback[T any](callback func(T) error, value T) error {
	if callback == nil {
		return nil
	}
	return callback(value)
}
