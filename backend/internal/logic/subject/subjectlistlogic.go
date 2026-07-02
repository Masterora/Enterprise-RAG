// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package subject

import (
	"context"
	"strings"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SubjectListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSubjectListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubjectListLogic {
	return &SubjectListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubjectListLogic) SubjectList(req *types.SubjectListReq) (resp *types.SubjectListResp, err error) {
	_, pageSize, offset := normalizePage(req.Page, req.PageSize)
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	subjects, total, err := l.svcCtx.SubjectRepo.ListAccessible(l.ctx, model.SubjectListFilter{
		UserID:   user.ID,
		Keyword:  strings.TrimSpace(req.Keyword),
		PageSize: pageSize,
		Offset:   offset,
	})
	if err != nil {
		return nil, err
	}

	list := make([]types.SubjectInfo, 0, len(subjects))
	for _, subject := range subjects {
		list = append(list, toSubjectInfo(subject))
	}

	return &types.SubjectListResp{
		List:  list,
		Total: total,
	}, nil
}
