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

type RetrievalSearchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRetrievalSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetrievalSearchLogic {
	return &RetrievalSearchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RetrievalSearchLogic) RetrievalSearch(req *types.RetrievalSearchReq) (resp *types.RetrievalSearchResp, err error) {
	subjectID := strings.TrimSpace(req.SubjectID)
	query := strings.TrimSpace(req.Query)
	if subjectID == "" || query == "" {
		return nil, errors.New("subject_id and query are required")
	}

	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}

	list, err := retrievalsvc.NewService(l.svcCtx).Search(l.ctx, user.ID, subjectID, query, req.TopK)
	if err != nil {
		return nil, err
	}

	return &types.RetrievalSearchResp{List: list}, nil
}
