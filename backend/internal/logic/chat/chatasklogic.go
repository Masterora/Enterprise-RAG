// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package chat

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/service/chatflow"
	retrievalsvc "enterprise-rag/backend/internal/service/retrieval"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

const noAnswer = "无法确定"

type ChatAskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatAskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatAskLogic {
	return &ChatAskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChatAskLogic) ChatAsk(req *types.ChatAskReq) (resp *types.ChatAskResp, err error) {
	query := strings.TrimSpace(req.Query)
	if strings.TrimSpace(req.SubjectID) == "" || query == "" {
		return nil, errors.New("subject_id and query are required")
	}

	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}

	route, routedAnswer, routedChunks, handled, err := chatflow.ResolveRoutedAnswer(l.ctx, l.svcCtx, user.ID, req.SubjectID, query, req.LlmProvider, req.LlmModel)
	if err != nil {
		logx.WithContext(l.ctx).Errorf("chat route failed: user_id=%s subject_id=%s route=%s err=%v", user.ID, req.SubjectID, route, err)
		return nil, err
	}
	if handled {
		answer := chatflow.NormalizeAnswerText(routedAnswer)
		if answer == "" {
			answer = noAnswer
		}
		if err := chatflow.PersistTurn(l.ctx, l.svcCtx, user.ID, req, answer, routedChunks, nil, types.RetrievalMetrics{}); err != nil {
			return nil, err
		}
		return &types.ChatAskResp{
			Answer:        answer,
			Chunks:        routedChunks,
			ExternalLinks: nil,
			Metrics:       types.RetrievalMetrics{},
		}, nil
	}

	chunks, metrics, err := retrievalsvc.NewService(l.svcCtx).SearchWithOptions(l.ctx, user.ID, req.SubjectID, query, retrievalsvc.SearchOptions{
		TopK:             req.TopK,
		ExpectedDocIDs:   req.ExpectedDocIDs,
		ExpectedChunkIDs: req.ExpectedChunkIDs,
		LLMProvider:      req.LlmProvider,
		LLMModel:         req.LlmModel,
	})
	if err != nil {
		logx.WithContext(l.ctx).Errorf("chat retrieval failed: user_id=%s subject_id=%s err=%v", user.ID, req.SubjectID, err)
		return nil, err
	}
	logx.WithContext(l.ctx).Infof("chat retrieval finished: user_id=%s subject_id=%s hits=%d recall_at_k=%.4f", user.ID, req.SubjectID, len(chunks), metrics.RecallAtK)

	if len(chunks) == 0 && !req.WebSearch {
		if err := chatflow.PersistTurn(l.ctx, l.svcCtx, user.ID, req, noAnswer, chunks, nil, metrics); err != nil {
			return nil, err
		}
		return &types.ChatAskResp{Answer: noAnswer, Chunks: chunks, ExternalLinks: nil, Metrics: metrics}, nil
	}

	llmClient, err := chatflow.ResolveLLM(l.ctx, l.svcCtx, req)
	if err != nil {
		logx.WithContext(l.ctx).Errorf("chat llm init failed: user_id=%s subject_id=%s provider=%s model=%s err=%v", user.ID, req.SubjectID, req.LlmProvider, req.LlmModel, err)
		return nil, err
	}

	var externalLinks []types.ExternalLink
	if req.WebSearch {
		searchQuery := strings.TrimSpace(metrics.SearchQuery)
		if searchQuery == "" {
			searchQuery = query
		}
		externalLinks, err = llmClient.SearchWeb(l.ctx, searchQuery)
		if err != nil {
			logx.WithContext(l.ctx).Errorf("chat web search failed: user_id=%s subject_id=%s err=%v", user.ID, req.SubjectID, err)
			return nil, err
		}
		logx.WithContext(l.ctx).Infof("chat web search finished: user_id=%s subject_id=%s links=%d", user.ID, req.SubjectID, len(externalLinks))
	}

	answer, err := llmClient.Generate(l.ctx, chatflow.BuildPrompt(l.svcCtx.Config.Prompt, query, chunks, externalLinks, req.WebSearch), false)
	if err != nil {
		logx.WithContext(l.ctx).Errorf("chat llm failed: user_id=%s subject_id=%s hits=%d err=%v", user.ID, req.SubjectID, len(chunks), err)
		return nil, err
	}
	answer = chatflow.NormalizeAnswerText(answer)
	if answer == "" {
		answer = noAnswer
	}
	logx.WithContext(l.ctx).Infof("chat llm finished: user_id=%s subject_id=%s hits=%d answer_chars=%d", user.ID, req.SubjectID, len(chunks), len([]rune(answer)))
	if err := chatflow.PersistTurn(l.ctx, l.svcCtx, user.ID, req, answer, chunks, externalLinks, metrics); err != nil {
		return nil, err
	}

	return &types.ChatAskResp{
		Answer:        answer,
		Chunks:        chunks,
		ExternalLinks: externalLinks,
		Metrics:       metrics,
	}, nil
}
