// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package subject

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/backend/internal/auth"
	subjectpresenter "enterprise-rag/backend/internal/presenter/subject"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SubjectDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSubjectDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubjectDetailLogic {
	return &SubjectDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubjectDetailLogic) SubjectDetail(req *types.SubjectDetailReq) (resp *types.SubjectDetailResp, err error) {
	id := strings.TrimSpace(req.ID)
	if id == "" {
		return nil, errors.New("knowledge base id is required")
	}

	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}

	subject, err := l.svcCtx.SubjectRepo.GetAccessibleByID(l.ctx, id, user.ID)
	if err != nil {
		return nil, err
	}

	return &types.SubjectDetailResp{
		Subject: subjectpresenter.ToInfo(*subject),
	}, nil
}
