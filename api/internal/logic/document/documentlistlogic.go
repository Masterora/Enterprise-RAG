// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package document

import (
	"context"
	"strings"

	"enterprise-rag/api/internal/auth"
	"enterprise-rag/api/internal/model"
	documentpresenter "enterprise-rag/api/internal/presenter/document"
	"enterprise-rag/api/internal/service/pagination"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DocumentListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDocumentListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DocumentListLogic {
	return &DocumentListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DocumentListLogic) DocumentList(req *types.DocumentListReq) (resp *types.DocumentListResp, err error) {
	_, pageSize, offset := pagination.Normalize(req.Page, req.PageSize)
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	documents, total, err := l.svcCtx.DocumentRepo.ListByUser(l.ctx, model.DocumentListFilter{
		UserID:    user.ID,
		TenantID:  user.TenantID,
		SubjectID: strings.TrimSpace(req.SubjectID),
		Status:    strings.TrimSpace(req.Status),
		Keyword:   strings.TrimSpace(req.Keyword),
		PageSize:  pageSize,
		Offset:    offset,
	})
	if err != nil {
		return nil, err
	}

	list := make([]types.DocumentInfo, 0, len(documents))
	for _, document := range documents {
		list = append(list, documentpresenter.ToInfo(document))
	}

	return &types.DocumentListResp{
		List:  list,
		Total: total,
	}, nil
}
