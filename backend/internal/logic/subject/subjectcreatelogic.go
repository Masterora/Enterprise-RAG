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
	subjectpresenter "enterprise-rag/backend/internal/presenter/subject"
	"enterprise-rag/backend/internal/repository"
	"enterprise-rag/backend/internal/service/subjectmeta"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

type SubjectCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSubjectCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SubjectCreateLogic {
	return &SubjectCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SubjectCreateLogic) SubjectCreate(req *types.SubjectCreateReq) (resp *types.SubjectCreateResp, err error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("knowledge base name is required")
	}

	id := uuid.NewString()
	now := time.Now()
	visibility := subjectmeta.NormalizeVisibility(req.Visibility)
	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}

	subject := &model.Subject{
		ID:          id,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		OwnerID:     user.ID,
		Visibility:  visibility,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := l.svcCtx.SubjectRepo.Create(l.ctx, subject); err != nil {
		if errors.Is(err, repository.ErrSubjectNameExists) {
			return nil, errors.New("knowledge base name already exists")
		}
		return nil, err
	}

	return &types.SubjectCreateResp{
		Subject: subjectpresenter.ToInfo(*subject),
	}, nil
}
