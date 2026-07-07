package auth

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

var errInvalidLanguage = errors.New("language is invalid")

type UserUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUserUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserUpdateLogic {
	return &UserUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserUpdateLogic) UserUpdate(req *types.UserUpdateReq) (*types.UserUpdateResp, error) {
	session, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}

	nickname := strings.TrimSpace(req.Nickname)
	email := strings.TrimSpace(req.Email)
	language := strings.TrimSpace(req.Language)
	if nickname == "" || email == "" {
		return nil, errors.New("nickname and email are required")
	}
	if language == "" {
		language = "zh-CN"
	}
	if language != "zh-CN" && language != "en-US" && language != "ja-JP" {
		return nil, errInvalidLanguage
	}

	user, err := l.svcCtx.UserRepo.UpdateProfile(l.ctx, session.ID, nickname, email, language)
	if err != nil {
		return nil, err
	}

	return &types.UserUpdateResp{
		User: types.UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Nickname: user.Nickname,
			Email:    user.Email,
			Language: user.Language,
		},
	}, nil
}
