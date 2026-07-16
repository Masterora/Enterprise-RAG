package postgres

import (
	"context"

	"enterprise-rag/api/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ModelSettingsRepo struct {
	db *pgxpool.Pool
}

func NewModelSettingsRepo(db *pgxpool.Pool) *ModelSettingsRepo {
	return &ModelSettingsRepo{db: db}
}

func (r *ModelSettingsRepo) GetByTenant(ctx context.Context, tenantID string) (*model.TenantModelSettings, error) {
	var settings model.TenantModelSettings
	err := r.db.QueryRow(
		ctx,
		`SELECT tenant_id::text, provider, api_key_ciphertext, api_key_hint, created_at, updated_at
		 FROM tenant_model_settings
		 WHERE tenant_id = $1`,
		tenantID,
	).Scan(
		&settings.TenantID,
		&settings.Provider,
		&settings.APIKeyCiphertext,
		&settings.APIKeyHint,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *ModelSettingsRepo) Upsert(ctx context.Context, settings *model.TenantModelSettings) error {
	_, err := r.db.Exec(
		ctx,
		`INSERT INTO tenant_model_settings (tenant_id, provider, api_key_ciphertext, api_key_hint, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, now(), now())
		 ON CONFLICT (tenant_id) DO UPDATE SET
		   provider = EXCLUDED.provider,
		   api_key_ciphertext = EXCLUDED.api_key_ciphertext,
		   api_key_hint = EXCLUDED.api_key_hint,
		   updated_at = now()`,
		settings.TenantID,
		settings.Provider,
		settings.APIKeyCiphertext,
		settings.APIKeyHint,
	)
	return err
}
