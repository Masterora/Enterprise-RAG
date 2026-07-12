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

type AdminTaskListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminTaskListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminTaskListLogic {
	return &AdminTaskListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminTaskListLogic) AdminTaskList(req *types.AdminTaskListReq) (resp *types.AdminTaskListResp, err error) {
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	_, pageSize, offset := pagination.Normalize(req.Page, req.PageSize)
	tasks, total, err := l.svcCtx.IndexTaskRepo.List(l.ctx, model.IndexTaskListFilter{
		UserID:    user.ID,
		SubjectID: strings.TrimSpace(req.SubjectID),
		Status:    strings.TrimSpace(req.Status),
		TaskType:  strings.TrimSpace(req.TaskType),
		PageSize:  pageSize,
		Offset:    offset,
	})
	if err != nil {
		return nil, err
	}
	list := make([]types.IndexTaskInfo, 0, len(tasks))
	for _, task := range tasks {
		list = append(list, documentpresenter.TaskToInfo(task))
	}

	return &types.AdminTaskListResp{List: list, Total: total}, nil
}
