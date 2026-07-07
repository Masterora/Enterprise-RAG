// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package chat

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/backend/internal/auth"
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

	chunks, err := retrievalsvc.NewService(l.svcCtx).Search(l.ctx, user.ID, req.SubjectID, query, req.TopK)
	if err != nil {
		logx.WithContext(l.ctx).Errorf("chat retrieval failed: user_id=%s subject_id=%s err=%v", user.ID, req.SubjectID, err)
		return nil, err
	}
	logx.WithContext(l.ctx).Infof("chat retrieval finished: user_id=%s subject_id=%s hits=%d", user.ID, req.SubjectID, len(chunks))

	if len(chunks) == 0 {
		return &types.ChatAskResp{Answer: noAnswer, Chunks: chunks}, nil
	}

	answer, err := l.svcCtx.LLM.Generate(l.ctx, buildPrompt(query, chunks))
	if err != nil {
		logx.WithContext(l.ctx).Errorf("chat llm failed: user_id=%s subject_id=%s hits=%d err=%v", user.ID, req.SubjectID, len(chunks), err)
		return nil, err
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		answer = noAnswer
	}
	logx.WithContext(l.ctx).Infof("chat llm finished: user_id=%s subject_id=%s hits=%d answer_chars=%d", user.ID, req.SubjectID, len(chunks), len([]rune(answer)))

	return &types.ChatAskResp{
		Answer: answer,
		Chunks: chunks,
	}, nil
}
