// Package gormdb is the GORM layer on top of engine/postgres: it opens a GORM
// connection from a postgres.Config, provides the schema-aware Database, the
// schema-routing Plugin and transaction helpers.
//
// The request schema is read from context through engine/scope; the package
// knows nothing about where it comes from (tenants, for example).
package gormdb

import (
	"gitlab.com/skyrix-lib/framework/logger"

	"errors"
	"fmt"

	"gitlab.com/skyrix-lib/framework/engine/postgres"

	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open validates the configuration and opens a GORM connection pool. It does
// not contact the server: the Component pings it on Start. The caller closes
// the pool.
//
// The pool is opened by postgres.OpenPool and handed to GORM as an existing
// connection, so GORM never parses the connection string and neither its
// errors nor its logger can contain the password.
func Open(cfg postgres.Config) (*gorm.DB, error) {
	sqlDB, err := postgres.OpenPool(cfg)
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// Provide opens the GORM pool for Wire and returns a cleanup that closes it.
// Add NewComponent to the binary's runtime so the connection is verified on
// start.
func Provide(cfg postgres.Config, log logger.Interface) (*gorm.DB, func(), error) {
	db, err := Open(cfg)
	if err != nil {
		log.Error("Unable to initialize postgres database", "error", err)
		return nil, nil, err
	}
	cleanup := func() {
		if sqlDB, err := db.DB(); err == nil {
			log.Info("Closing postgres database connection")
			_ = sqlDB.Close()
		}
	}
	return db, cleanup, nil
}

// NewComponent returns the postgres lifecycle component for the pool behind
// db.
func NewComponent(db *gorm.DB) (*postgres.Component, error) {
	if db == nil {
		return nil, errors.New("gormdb: db is nil")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("gormdb: %w", err)
	}
	return postgres.NewComponent(sqlDB), nil
}
