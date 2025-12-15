package repository

import (
	"context"

	"skyrix/internal/engine"
	"skyrix/internal/engine/tenantPackage/entity"
)

type TenantRepository struct {
	DB *engine.Database
}

func NewTenantRepository(db *engine.Database) *TenantRepository {
	return &TenantRepository{DB: db}
}

func (r *TenantRepository) GetByNamespace(ctx context.Context, ns string) (*entity.Tenant, error) {
	var t entity.Tenant
	err := r.DB.WithContext(ctx).
		// MAIN schema request (no tenant ctx) -> WithContext will set main schema search_path
		Where("tenant = ?", ns).
		First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TenantRepository) GetByDomain(ctx context.Context, domain string) (*entity.Tenant, error) {
	var t entity.Tenant
	err := r.DB.WithContext(ctx).
		Where("domain = ?", domain).
		First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListDomains For CORS (optional): return list of domains from main table
func (r *TenantRepository) ListDomains(ctx context.Context) ([]string, error) {
	var out []string
	err := r.DB.WithContext(ctx).
		Model(&entity.Tenant{}).
		Select("domain").
		Where("is_active = true AND domain IS NOT NULL AND domain <> ''").
		Scan(&out).Error
	return out, err
}
