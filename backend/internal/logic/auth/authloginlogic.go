// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

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

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuthLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuthLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthLoginLogic {
	return &AuthLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AuthLoginLogic) AuthLogin(req *types.AuthLoginReq) (resp *types.AuthLoginResp, err error) {
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		return nil, errors.New("username and password are required")
	}

	user, err := l.svcCtx.UserRepo.FindByUsername(l.ctx, username)
	if errors.Is(err, pgrepo.ErrUserNotFound) {
		hashed, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return nil, hashErr
		}
		now := time.Now()
		user = &model.User{
			ID:           uuid.NewString(),
			Username:     username,
			PasswordHash: string(hashed),
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if createErr := l.svcCtx.UserRepo.Create(l.ctx, user); createErr != nil {
			return nil, createErr
		}
	} else if err != nil {
		return nil, err
	} else if compareErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); compareErr != nil {
		return nil, errors.New("username or password is invalid")
	}

	token, err := auth.GenerateToken(l.svcCtx.Config.Auth.AccessSecret, l.svcCtx.Config.Auth.ExpireHours, auth.UserSession{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	})
	if err != nil {
		return nil, err
	}

	return &types.AuthLoginResp{
		Token: token,
		User: types.UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
		},
	}, nil
}
