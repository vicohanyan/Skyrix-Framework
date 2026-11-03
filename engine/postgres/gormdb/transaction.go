package gormdb

import (
	"context"

	"gitlab.com/skyrix-lib/framework/logger"
	"gorm.io/gorm"
)

// TxManager is the legacy raw-GORM transaction adapter.
// Deprecated: use Database.WithinTransaction and pass its Database to WithTx.
type TxManager struct {
	DB     *Database
	Logger logger.Interface
}

// NewTxManager constructs the legacy adapter.
// Deprecated: use Database.WithinTransaction.
func NewTxManager(db *Database, log logger.Interface) *TxManager {
	return &TxManager{DB: db, Logger: log}
}

// Execute delegates commit/rollback/panic to WithinTransaction. Raw SQL inherits
// SET LOCAL, but a raw *gorm.DB cannot enforce Database.WithContext scope checks
// if callers replace its context. Migrate repository WithTx to accept *Database.
// Logger is retained for source compatibility; transaction logging is GORM's job.
func (tm *TxManager) Execute(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return tm.DB.WithinTransaction(ctx, func(tx *Database) error { return fn(tx.DB) })
}
