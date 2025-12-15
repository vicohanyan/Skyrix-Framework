package engine

import (
	"context"
	"skyrix/internal/logger"

	"gorm.io/gorm"
)

type TxManager struct {
	DB     *Database
	Logger logger.Interface
}

// NewTxManager constructs a TransactionManager backed by the provided Database and logger.
func NewTxManager(db *Database, logger logger.Interface) TransactionManager {
	return &TxManager{
		DB:     db,
		Logger: logger,
	}
}

// Execute wraps fn in a database transaction with rollback on error or panic.
func (tm *TxManager) Execute(ctx context.Context, fn func(tx *gorm.DB) error) error {
	tx := tm.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		tm.Logger.Error("Transaction failed to begin", "error", tx.Error)
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tm.Logger.Error("Transaction rolled back due to panic", "panic_value", r)
			tx.Rollback()
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		tm.Logger.Error("Transaction rolled back due to function error", "error", err)
		tx.Rollback()
		return err
	}

	commitErr := tx.Commit().Error
	if commitErr != nil {
		tm.Logger.Error("Transaction commit failed", "error", commitErr)
	}
	return commitErr
}
