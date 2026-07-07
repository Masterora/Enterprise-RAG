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

type AuthRegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuthRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthRegisterLogic {
	return &AuthRegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AuthRegisterLogic) AuthRegister(req *types.AuthRegisterReq) (*types.AuthLoginResp, error) {
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	confirmPassword := strings.TrimSpace(req.ConfirmPassword)
	nickname := strings.TrimSpace(req.Nickname)
	email := strings.TrimSpace(req.Email)

	if username == "" || password == "" {
		return nil, errors.New("username and password are required")
	}
	if nickname == "" || email == "" {
		return nil, errors.New("nickname and email are required")
	}
	if password != confirmPassword {
		return nil, errors.New("password confirmation does not match")
	}

	_, err := l.svcCtx.UserRepo.FindByUsername(l.ctx, username)
	if err == nil {
		return nil, errors.New("username already exists")
	}
	if !errors.Is(err, pgrepo.ErrUserNotFound) {
		return nil, err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user := &model.User{
		ID:           uuid.NewString(),
		Username:     username,
		Nickname:     nickname,
		Email:        email,
		Language:     "zh-CN",
		PasswordHash: string(hashed),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := l.svcCtx.UserRepo.Create(l.ctx, user); err != nil {
		return nil, err
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
