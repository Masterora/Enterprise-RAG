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
	visibility := normalizeVisibility(req.Visibility)

	subject := &model.Subject{
		ID:          id,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		OwnerID:     auth.MockCurrentUserID,
		Visibility:  visibility,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := l.svcCtx.SubjectRepo.Create(l.ctx, subject); err != nil {
		return nil, err
	}

	return &types.SubjectCreateResp{
		Subject: toSubjectInfo(*subject),
	}, nil
}
