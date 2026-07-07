// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"
	"errors"
	"strings"

	pgrepo "enterprise-rag/backend/internal/repository/postgres"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"enterprise-rag/backend/internal/auth"

	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/crypto/bcrypt"
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
		return nil, errors.New("username or password is invalid")
	}
	if err != nil {
		return nil, err
	}
	if compareErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); compareErr != nil {
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
			Nickname: user.Nickname,
			Email:    user.Email,
			Language: user.Language,
		},
	}, nil
}
