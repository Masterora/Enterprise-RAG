// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package retrieval

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

type RetrievalEvaluateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRetrievalEvaluateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetrievalEvaluateLogic {
	return &RetrievalEvaluateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RetrievalEvaluateLogic) RetrievalEvaluate(req *types.RetrievalEvaluateReq) (*types.RetrievalEvaluateResp, error) {
	subjectID := strings.TrimSpace(req.SubjectID)
	if subjectID == "" {
		return nil, errors.New("subject_id is required")
	}
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	return retrievalsvc.NewEvaluator(l.svcCtx).Run(l.ctx, user.ID, subjectID)
}
