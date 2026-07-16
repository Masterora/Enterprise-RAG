// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package subject

import (
	"context"
	"errors"
	"strings"
	"time"

	"enterprise-rag/api/internal/auth"
	"enterprise-rag/api/internal/model"
	subjectpresenter "enterprise-rag/api/internal/presenter/subject"
	"enterprise-rag/api/internal/repository"
	pgrepo "enterprise-rag/api/internal/repository/postgres"
	"enterprise-rag/api/internal/service/subjectmeta"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

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
		TenantID:    user.TenantID,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		OwnerID:     user.ID,
		Visibility:  subjectmeta.NormalizeVisibility(req.Visibility),
		UpdatedAt:   now,
	}
	if err := l.svcCtx.SubjectRepo.UpdateByOwner(l.ctx, subject); err != nil {
		if errors.Is(err, repository.ErrSubjectNameExists) {
			return nil, errors.New("knowledge base name already exists")
		}
		if errors.Is(err, pgrepo.ErrSubjectNotFound) {
			return nil, errors.New("knowledge base not found")
		}
		return nil, err
	}

	updated, err := l.svcCtx.SubjectRepo.GetAccessibleByID(l.ctx, id, user.ID, user.TenantID)
	if err != nil {
		return nil, err
	}

	return &types.SubjectUpdateResp{
		Subject: subjectpresenter.ToInfo(*updated),
	}, nil
}
