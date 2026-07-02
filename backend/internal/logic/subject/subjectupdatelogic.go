// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package subject

import (
	"context"
	"errors"
	"strings"
	"time"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/model"
	pgrepo "enterprise-rag/backend/internal/repository/postgres"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SubjectUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSubjectUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubjectUpdateLogic {
	return &SubjectUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubjectUpdateLogic) SubjectUpdate(req *types.SubjectUpdateReq) (resp *types.SubjectUpdateResp, err error) {
	id := strings.TrimSpace(req.ID)
	name := strings.TrimSpace(req.Name)
	if id == "" || name == "" {
		return nil, errors.New("id and name are required")
	}

	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	subject := &model.Subject{
		ID:          id,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		OwnerID:     user.ID,
		Visibility:  normalizeVisibility(req.Visibility),
		UpdatedAt:   now,
	}
	if err := l.svcCtx.SubjectRepo.UpdateByOwner(l.ctx, subject); err != nil {
		if errors.Is(err, pgrepo.ErrSubjectNotFound) {
			return nil, errors.New("knowledge base not found")
		}
		return nil, err
	}

	updated, err := l.svcCtx.SubjectRepo.GetAccessibleByID(l.ctx, id, user.ID)
	if err != nil {
		return nil, err
	}

	return &types.SubjectUpdateResp{
		Subject: toSubjectInfo(*updated),
	}, nil
}
