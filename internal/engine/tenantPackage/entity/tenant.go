package entity

import (
	"skyrix/internal/kernel/db/scope"
	"time"
)

// Tenant lives in MAIN schema (core).
type Tenant struct {
	scope.MainModel

	ID        int64      `gorm:"column:id;primaryKey"`
	Namespace string     `gorm:"column:tenant;type:text;not null;index:ux_tenant_alive,unique,where:deleted_at IS NULL"`
	Schema    *string    `gorm:"column:schema;type:text;default:null;index:uniq_subscriber_schema_nz,unique,where:deleted_at IS NULL"`
	Domain    *string    `gorm:"column:domain;type:text;default:null;index:uniq_subscriber_domain_nz,unique,where:deleted_at IS NULL"`
	IsActive  bool       `gorm:"column:is_active"`
	ActiveTo  *time.Time `gorm:"column:active_to;index"`
	UpdatedAt time.Time  `gorm:"column:updated_at"`
}

func (Tenant) TableName() string { return "tenants" }
