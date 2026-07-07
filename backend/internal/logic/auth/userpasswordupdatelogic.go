package auth

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"golang.org/x/crypto/bcrypt"

	"github.com/zeromicro/go-zero/core/logx"
)

var (
	errOldPasswordRequired  = errors.New("old password is required")
	errNewPasswordRequired  = errors.New("new password is required")
	errPasswordMismatch     = errors.New("password confirmation does not match")
	errOldPasswordIncorrect = errors.New("old password is incorrect")
)

type UserPasswordUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUserPasswordUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserPasswordUpdateLogic {
	return &UserPasswordUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserPasswordUpdateLogic) UserPasswordUpdate(req *types.UserPasswordUpdateReq) (*types.UserPasswordUpdateResp, error) {
	session, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}

	oldPassword := strings.TrimSpace(req.OldPassword)
	newPassword := strings.TrimSpace(req.NewPassword)
	confirmPassword := strings.TrimSpace(req.ConfirmPassword)

	if oldPassword == "" {
		return nil, errOldPasswordRequired
	}
	if newPassword == "" {
		return nil, errNewPasswordRequired
	}
	if newPassword != confirmPassword {
		return nil, errPasswordMismatch
	}

	user, err := l.svcCtx.UserRepo.GetByID(l.ctx, session.ID)
	if err != nil {
		return nil, err
	}

	if compareErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); compareErr != nil {
		return nil, errOldPasswordIncorrect
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	if err := l.svcCtx.UserRepo.UpdatePassword(l.ctx, session.ID, string(hashed)); err != nil {
		return nil, err
	}

	return &types.UserPasswordUpdateResp{}, nil
}
