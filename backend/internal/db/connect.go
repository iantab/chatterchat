package db

import (
	"context"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	dbOnce  sync.Once
	dbPool  *sqlx.DB
	dbError error
)

// Get returns the singleton DB connection pool, initializing it on first call.
func Get(ctx context.Context) (*sqlx.DB, error) {
	dbOnce.Do(func() {
		dsn, err := GetDSN(ctx)
		if err != nil {
			dbError = fmt.Errorf("get DSN: %w", err)
			return
		}

		pool, err := sqlx.ConnectContext(ctx, "postgres", dsn)
		if err != nil {
			dbError = fmt.Errorf("connect to postgres: %w", err)
			return
		}

		pool.SetMaxOpenConns(5)
		pool.SetMaxIdleConns(5)
		dbPool = pool
	})
	return dbPool, dbError
}
