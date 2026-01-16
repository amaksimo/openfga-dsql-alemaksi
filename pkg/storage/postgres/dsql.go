package postgres

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/aws-samples/aurora-dsql-samples/go/dsql-pgx-connector/dsql"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openfga/openfga/pkg/storage/sqlcommon"
)

// isOCCError checks if the error is a DSQL optimistic concurrency control conflict.
// DSQL returns OC000 for mutation conflicts and OC001 for schema conflicts.
func isOCCError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "OC000") || strings.Contains(msg, "OC001") || strings.Contains(msg, "40001")
}

// withOCCRetry executes fn with automatic retry on DSQL OCC errors.
// Uses exponential backoff with jitter between retries.
func withOCCRetry(fn func() error) error {
	const maxRetries = 5
	var err error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err = fn(); err == nil || !isOCCError(err) {
			return err
		}
		base := time.Duration(10<<attempt) * time.Millisecond
		time.Sleep(base + time.Duration(rand.Int63n(int64(base/2))))
	}
	return fmt.Errorf("OCC retry limit exceeded: %w", err)
}

// initDSQLDB initializes a new Aurora DSQL database connection.
// DSQL uses IAM authentication which the connector handles automatically.
func initDSQLDB(uri string, cfg *sqlcommon.Config) (*pgxpool.Pool, error) {
	dsqlCfg, err := dsql.ParseConnectionString(uri)
	if err != nil {
		return nil, fmt.Errorf("parse DSQL URI: %w", err)
	}

	// Apply OpenFGA pool settings
	if cfg.MaxOpenConns != 0 {
		dsqlCfg.MaxConns = int32(cfg.MaxOpenConns)
	}
	if cfg.MinOpenConns != 0 {
		dsqlCfg.MinConns = int32(cfg.MinOpenConns)
	}
	if cfg.ConnMaxLifetime != 0 {
		dsqlCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	}
	if cfg.ConnMaxIdleTime != 0 {
		dsqlCfg.MaxConnIdleTime = cfg.ConnMaxIdleTime
	}

	pool, err := dsql.NewPool(context.Background(), dsqlCfg)
	if err != nil {
		return nil, fmt.Errorf("create DSQL pool: %w", err)
	}

	return pool.Pool, nil
}
