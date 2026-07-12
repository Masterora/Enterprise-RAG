// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminSummaryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminSummaryLogic {
	return &AdminSummaryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminSummaryLogic) AdminSummary() (resp *types.AdminSummaryResp, err error) {
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	summary, err := l.svcCtx.AdminRepo.Summary(l.ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &types.AdminSummaryResp{
		SubjectTotal:     summary.SubjectTotal,
		DocumentTotal:    summary.DocumentTotal,
		ChunkTotal:       summary.ChunkTotal,
		SessionTotal:     summary.SessionTotal,
		IndexedTotal:     summary.IndexedTotal,
		ProcessingTotal:  summary.ProcessingTotal,
		FailedTotal:      summary.FailedTotal,
		PendingTaskTotal: summary.PendingTaskTotal,
		RunningTaskTotal: summary.RunningTaskTotal,
		FailedTaskTotal:  summary.FailedTaskTotal,
	}, nil
}
