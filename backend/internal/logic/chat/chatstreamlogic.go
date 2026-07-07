package chat

import (
	"errors"
	"strings"

	"enterprise-rag/backend/internal/auth"
	retrievalsvc "enterprise-rag/backend/internal/service/retrieval"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type StreamCallbacks struct {
	OnStatus  func(string) error
	OnSources func([]types.RetrievalChunk) error
	OnDelta   func(string) error
	OnDone    func() error
}

func (l *ChatAskLogic) ChatStream(req *types.ChatAskReq, callbacks StreamCallbacks) error {
	query := strings.TrimSpace(req.Query)
	if strings.TrimSpace(req.SubjectID) == "" || query == "" {
		return errors.New("subject_id and query are required")
	}

	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return err
	}

	if callbacks.OnStatus != nil {
		if err := callbacks.OnStatus("正在检索知识库..."); err != nil {
			return err
		}
	}

	chunks, err := retrievalsvc.NewService(l.svcCtx).Search(l.ctx, user.ID, req.SubjectID, query, req.TopK)
	if err != nil {
		logx.WithContext(l.ctx).Errorf("chat stream retrieval failed: user_id=%s subject_id=%s err=%v", user.ID, req.SubjectID, err)
		return err
	}
	logx.WithContext(l.ctx).Infof("chat stream retrieval finished: user_id=%s subject_id=%s hits=%d", user.ID, req.SubjectID, len(chunks))

	if callbacks.OnSources != nil {
		if err := callbacks.OnSources(chunks); err != nil {
			return err
		}
	}

	if len(chunks) == 0 {
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
		if callbacks.OnDone != nil {
			return callbacks.OnDone()
		}
		return nil
	}

	if callbacks.OnStatus != nil {
		if err := callbacks.OnStatus("已找到相关片段，正在生成答案..."); err != nil {
			return err
		}
	}

	var answerBuilder strings.Builder
	err = l.svcCtx.LLM.GenerateStream(l.ctx, buildPrompt(query, chunks), func(delta string) error {
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

	answer := strings.TrimSpace(answerBuilder.String())
	if answer == "" && callbacks.OnDelta != nil {
		if err := callbacks.OnDelta(noAnswer); err != nil {
			return err
		}
	}
	logx.WithContext(l.ctx).Infof("chat stream llm finished: user_id=%s subject_id=%s hits=%d answer_chars=%d", user.ID, req.SubjectID, len(chunks), len([]rune(answer)))

	if callbacks.OnDone != nil {
		return callbacks.OnDone()
	}
	return nil
}
