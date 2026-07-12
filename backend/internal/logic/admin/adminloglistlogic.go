// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"strings"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/model"
	documentpresenter "enterprise-rag/backend/internal/presenter/document"
	"enterprise-rag/backend/internal/service/pagination"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminLogListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminLogListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminLogListLogic {
	return &AdminLogListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminLogListLogic) AdminLogList(req *types.AdminLogListReq) (resp *types.AdminLogListResp, err error) {
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	_, pageSize, offset := pagination.Normalize(req.Page, req.PageSize)
	logs, total, err := l.svcCtx.DocumentRepo.ListParseLogs(l.ctx, model.ParseLogListFilter{
		UserID:    user.ID,
		SubjectID: strings.TrimSpace(req.SubjectID),
		Status:    strings.TrimSpace(req.Status),
		PageSize:  pageSize,
		Offset:    offset,
	})
	if err != nil {
		return nil, err
	}
	list := make([]types.ParseLogInfo, 0, len(logs))
	for _, item := range logs {
		list = append(list, documentpresenter.LogToInfo(item))
	}

	return &types.AdminLogListResp{List: list, Total: total}, nil
}
