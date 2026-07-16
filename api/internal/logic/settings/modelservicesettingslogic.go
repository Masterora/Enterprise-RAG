// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package settings

import (
	"context"
	"errors"
	"strings"
	"time"

	"enterprise-rag/api/internal/auth"
	"enterprise-rag/api/internal/config"
	"enterprise-rag/api/internal/infrastructure/embedding"
	"enterprise-rag/api/internal/service/modelsettings"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

var ErrInvalidAPIKey = errors.New("API Key 校验失败，请确认 Key 有效且账户额度充足")

type ModelServiceSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewModelServiceSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ModelServiceSettingsLogic {
	return &ModelServiceSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ModelServiceSettingsLogic) ModelServiceSettings() (*types.ModelServiceSettingsResp, error) {
	session, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	status, err := l.svcCtx.ModelSettings.Status(l.ctx, session.TenantID)
	if err != nil {
		return nil, err
	}
	return l.response(status), nil
}

func (l *ModelServiceSettingsLogic) Update(req *types.ModelServiceSettingsUpdateReq) (*types.ModelServiceSettingsResp, error) {
	session, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if len(apiKey) < 8 || !modelsettings.IsConfiguredSecret(apiKey) {
		return nil, ErrInvalidAPIKey
	}
	validationCtx, cancel := context.WithTimeout(l.ctx, 25*time.Second)
	defer cancel()
	if err := l.svcCtx.Embedder.ValidateAPIKey(validationCtx, apiKey); err != nil {
		l.Errorf("model API key validation failed for tenant %s: %v", session.TenantID, sanitizeValidationError(err))
		return nil, ErrInvalidAPIKey
	}
	if err := l.svcCtx.ModelSettings.Save(l.ctx, session.TenantID, apiKey); err != nil {
		return nil, err
	}
	status, err := l.svcCtx.ModelSettings.Status(l.ctx, session.TenantID)
	if err != nil {
		return nil, err
	}
	return l.response(status), nil
}

func (l *ModelServiceSettingsLogic) response(status modelsettings.Status) *types.ModelServiceSettingsResp {
	embedding := l.svcCtx.Config.Embedding
	return &types.ModelServiceSettingsResp{
		Provider:            embeddingProvider(embedding),
		BaseURL:             embedding.BaseURL,
		EmbeddingModel:      embedding.Model,
		EmbeddingDimension:  embedding.Dimension,
		APIKeyConfigured:    status.Configured,
		APIKeyHint:          status.APIKeyHint,
		ConfigurationSource: status.Source,
	}
}

func embeddingProvider(conf config.EmbeddingConf) string {
	return embedding.ProviderName(conf)
}

func sanitizeValidationError(err error) string {
	message := strings.ReplaceAll(err.Error(), "\n", " ")
	if len(message) > 300 {
		return message[:300]
	}
	return message
}
