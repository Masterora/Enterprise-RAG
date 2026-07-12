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

type StreamCallbacks struct {
	OnStatus     func(string) error
	OnSources    func([]types.RetrievalChunk) error
	OnWebSources func([]types.ExternalLink) error
	OnMetrics    func(types.RetrievalMetrics) error
	OnDelta      func(string) error
	OnDone       func() error
}

type ChatStreamLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewChatStreamLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ChatStreamLogic {
	return &ChatStreamLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ChatStreamLogic) ChatStream(req *types.ChatAskReq, callbacks StreamCallbacks) error {
	query := strings.TrimSpace(req.Query)
	if strings.TrimSpace(req.SubjectID) == "" || query == "" {
		return errors.New("subject_id and query are required")
	}

	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return err
	}

	route, routedAnswer, routedChunks, handled, err := chatflow.ResolveRoutedAnswer(l.ctx, l.svcCtx, user.ID, req.SubjectID, query, req.LlmProvider, req.LlmModel)
	if err != nil {
		logx.WithContext(l.ctx).Errorf("chat stream route failed: user_id=%s subject_id=%s route=%s err=%v", user.ID, req.SubjectID, route, err)
		return err
	}
	if handled {
		if callbacks.OnStatus != nil {
			status := "正在整理知识库内容..."
			if route == chatflow.QueryRouteNavigation {
				status = "正在整理相关文档..."
			}
			if err := callbacks.OnStatus(status); err != nil {
				return err
			}
		}
		if callbacks.OnSources != nil {
			if err := callbacks.OnSources(routedChunks); err != nil {
				return err
			}
		}
		answer := chatflow.NormalizeAnswerText(routedAnswer)
		if answer == "" {
			answer = noAnswer
		}
		if callbacks.OnDelta != nil {
			if err := callbacks.OnDelta(answer); err != nil {
				return err
			}
		}
		if err := chatflow.PersistTurn(l.ctx, l.svcCtx, user.ID, req, answer, routedChunks, nil, types.RetrievalMetrics{}); err != nil {
			return err
		}
		if callbacks.OnDone != nil {
			return callbacks.OnDone()
		}
		return nil
	}

	if callbacks.OnStatus != nil {
		if err := callbacks.OnStatus("正在检索知识库..."); err != nil {
			return err
		}
	}

	chunks, metrics, err := retrievalsvc.NewService(l.svcCtx).SearchWithOptions(l.ctx, user.ID, req.SubjectID, query, retrievalsvc.SearchOptions{
		TopK:             req.TopK,
		ExpectedDocIDs:   req.ExpectedDocIDs,
		ExpectedChunkIDs: req.ExpectedChunkIDs,
		LLMProvider:      req.LlmProvider,
		LLMModel:         req.LlmModel,
		OnStage:          callbacks.OnStatus,
	})
	if err != nil {
		logx.WithContext(l.ctx).Errorf("chat stream retrieval failed: user_id=%s subject_id=%s err=%v", user.ID, req.SubjectID, err)
		return err
	}
	logx.WithContext(l.ctx).Infof("chat stream retrieval finished: user_id=%s subject_id=%s hits=%d recall_at_k=%.4f", user.ID, req.SubjectID, len(chunks), metrics.RecallAtK)

	if callbacks.OnMetrics != nil {
		if err := callbacks.OnMetrics(metrics); err != nil {
			return err
		}
	}

	if callbacks.OnSources != nil {
		if err := callbacks.OnSources(chunks); err != nil {
			return err
		}
	}

	if len(chunks) == 0 && !req.WebSearch {
		if callbacks.OnStatus != nil {
			if err := callbacks.OnStatus("资料不足，无法确定答案"); err != nil {
				return err
			}
		}
		if callbacks.OnDelta != nil {
			if err := callbacks.OnDelta(noAnswer); err != nil {
				return err
			}
		}
		if err := chatflow.PersistTurn(l.ctx, l.svcCtx, user.ID, req, noAnswer, chunks, nil, metrics); err != nil {
			return err
		}
		if callbacks.OnDone != nil {
			return callbacks.OnDone()
		}
		return nil
	}

	if callbacks.OnStatus != nil {
		status := "已找到相关片段，正在生成答案..."
		if req.WebSearch {
			status = "正在通过网络搜索补充信息..."
		}
		if err := callbacks.OnStatus(status); err != nil {
			return err
		}
	}

	llmClient, err := chatflow.ResolveLLM(l.ctx, l.svcCtx, req)
	if err != nil {
		logx.WithContext(l.ctx).Errorf("chat stream llm init failed: user_id=%s subject_id=%s provider=%s model=%s err=%v", user.ID, req.SubjectID, req.LlmProvider, req.LlmModel, err)
		return err
	}

	var externalLinks []types.ExternalLink
	if req.WebSearch {
		if callbacks.OnStatus != nil {
			if err := callbacks.OnStatus("正在联网搜索外部资料..."); err != nil {
				return err
			}
		}
		searchQuery := strings.TrimSpace(metrics.SearchQuery)
		if searchQuery == "" {
			searchQuery = query
		}
		externalLinks, err = llmClient.SearchWeb(l.ctx, searchQuery)
		if err != nil {
			logx.WithContext(l.ctx).Errorf("chat stream web search failed: user_id=%s subject_id=%s err=%v", user.ID, req.SubjectID, err)
			return err
		}
		if callbacks.OnWebSources != nil {
			if err := callbacks.OnWebSources(externalLinks); err != nil {
				return err
			}
		}
		if callbacks.OnStatus != nil {
			if len(externalLinks) > 0 {
				if err := callbacks.OnStatus("已获取外部网页资料，正在生成答案..."); err != nil {
					return err
				}
			} else {
				if err := callbacks.OnStatus("未找到可用外部资料，正在基于知识库生成答案..."); err != nil {
					return err
				}
			}
		}
	}

	var answerBuilder strings.Builder
	err = llmClient.GenerateStream(l.ctx, chatflow.BuildPrompt(l.svcCtx.Config.Prompt, query, chunks, externalLinks, req.WebSearch), false, func(delta string) error {
		answerBuilder.WriteString(delta)
		if callbacks.OnDelta == nil {
			return nil
		}
		return callbacks.OnDelta(delta)
	})
	if err != nil {
		logx.WithContext(l.ctx).Errorf("chat stream llm failed: user_id=%s subject_id=%s hits=%d err=%v", user.ID, req.SubjectID, len(chunks), err)
		return err
	}

	answer := chatflow.NormalizeAnswerText(answerBuilder.String())
	if answer == "" {
		answer = noAnswer
		if callbacks.OnDelta != nil {
			if err := callbacks.OnDelta(answer); err != nil {
				return err
			}
		}
	}
	logx.WithContext(l.ctx).Infof("chat stream llm finished: user_id=%s subject_id=%s hits=%d answer_chars=%d", user.ID, req.SubjectID, len(chunks), len([]rune(answer)))
	if err := chatflow.PersistTurn(l.ctx, l.svcCtx, user.ID, req, answer, chunks, externalLinks, metrics); err != nil {
		return err
	}

	if callbacks.OnDone != nil {
		return callbacks.OnDone()
	}
	return nil
}
