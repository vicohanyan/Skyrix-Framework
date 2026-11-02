package entity

import (
	"gitlab.com/skyrix-lib/framework/engine/scope"
	"time"
)

// Tenant lives in MAIN schema (core). The application owns the tenants table
// (see extensions/tenant/README.md); the framework only reads tenants.
// Availability is expressed by IsActive and ActiveTo, there is no soft delete.
//
// The index names ux_tenant_alive, uniq_subscriber_schema_nz and
// uniq_subscriber_domain_nz are kept on purpose for compatibility with
// existing databases: AutoMigrate recognises indexes by name, so renaming
// them would create duplicate unique indexes. PostgreSQL unique indexes
// ignore NULL, so Schema and Domain stay unique only when set.
type Tenant struct {
	scope.MainModel

	ID        int64      `gorm:"column:id;primaryKey"`
	Namespace string     `gorm:"column:tenant;type:text;not null;index:ux_tenant_alive,unique"`
	Schema    *string    `gorm:"column:schema;type:text;default:null;index:uniq_subscriber_schema_nz,unique"`
	Domain    *string    `gorm:"column:domain;type:text;default:null;index:uniq_subscriber_domain_nz,unique"`
	IsActive  bool       `gorm:"column:is_active"`
	ActiveTo  *time.Time `gorm:"column:active_to;index"`
	UpdatedAt time.Time  `gorm:"column:updated_at"`
}

func (Tenant) TableName() string { return "tenants" }
