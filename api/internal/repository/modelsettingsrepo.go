package repository

import (
	"context"

	"enterprise-rag/api/internal/model"
)

type ModelSettingsRepository interface {
	GetByTenant(ctx context.Context, tenantID string) (*model.TenantModelSettings, error)
	Upsert(ctx context.Context, settings *model.TenantModelSettings) error
}
