// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package subject

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/api/internal/auth"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

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

	if err := l.svcCtx.MilvusStore.DeleteBySubject(l.ctx, user.ID, id); err != nil {
		logx.WithContext(l.ctx).Errorf("subject delete milvus delete failed: user_id=%s subject_id=%s err=%v", user.ID, id, err)
		return nil, err
	}

	deleted, err := l.svcCtx.SubjectRepo.SoftDeleteByOwner(l.ctx, id, user.ID, user.TenantID)
	if err != nil {
		return nil, err
	}
	logx.WithContext(l.ctx).Infof("subject delete finished: user_id=%s subject_id=%s deleted=%t", user.ID, id, deleted)

	return &types.SubjectDeleteResp{Deleted: deleted}, nil
}
