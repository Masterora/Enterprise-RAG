package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"enterprise-rag/api/internal/auth"
	"enterprise-rag/api/internal/model"
	chatpresenter "enterprise-rag/api/internal/presenter/chat"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

	"github.com/google/uuid"
)

func PersistTurn(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	userID string,
	req *types.ChatAskReq,
	answer string,
	chunks []types.RetrievalChunk,
	externalLinks []types.ExternalLink,
	metrics types.RetrievalMetrics,
	agentSteps []types.AgentStep,
) error {
	sessionID := strings.TrimSpace(req.SessionID)
	messageID := strings.TrimSpace(req.MessageID)
	if sessionID == "" && messageID == "" {
		return nil
	}
	if _, err := uuid.Parse(sessionID); err != nil {
		return errors.New("invalid session id")
	}
	if _, err := uuid.Parse(messageID); err != nil {
		return errors.New("invalid message id")
	}
	citations, err := json.Marshal(chunks)
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(chatpresenter.MessageMetadata{
		Metrics: metrics, ModelLabel: req.LlmModel, ModelID: req.LlmModel, WebSearch: req.WebSearch,
		ExternalLinks: externalLinks, AgentSteps: agentSteps,
	})
	if err != nil {
		return err
	}
	now := time.Now()
	return svcCtx.ChatRepo.SaveTurn(ctx, &model.ChatSession{
		ID: sessionID, TenantID: tenantIDFromContext(ctx), UserID: userID, SubjectID: req.SubjectID, Title: BuildSessionTitle(req.Query),
		LLMProvider: req.LlmProvider, LLMModel: req.LlmModel,
	}, &model.ChatMessage{
		ID: messageID, TenantID: tenantIDFromContext(ctx), SessionID: sessionID, UserID: userID, Question: req.Query,
		Answer: answer, Citations: citations, Metadata: metadata, CreatedAt: now,
	})
}

func tenantIDFromContext(ctx context.Context) string {
	user, err := auth.CurrentUser(ctx)
	if err != nil {
		return ""
	}
	return user.TenantID
}

func BuildSessionTitle(question string) string {
	runes := []rune(strings.TrimSpace(question))
	if len(runes) <= 18 {
		return string(runes)
	}
	return string(runes[:18]) + "..."
}
