// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package subject

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SubjectDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSubjectDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubjectDeleteLogic {
	return &SubjectDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubjectDeleteLogic) SubjectDelete(req *types.SubjectDeleteReq) (resp *types.SubjectDeleteResp, err error) {
	id := strings.TrimSpace(req.ID)
	if id == "" {
		return nil, errors.New("knowledge base id is required")
	}

	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}

	deleted, err := l.svcCtx.SubjectRepo.SoftDeleteByOwner(l.ctx, id, user.ID)
	if err != nil {
		return nil, err
	}

	return &types.SubjectDeleteResp{Deleted: deleted}, nil
}
