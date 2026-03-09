package sqlxw

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/jmoiron/sqlx"
)

type contextKey string

const txKey contextKey = "sqlxw-tx"

// DriverType defines the supported SQL databases for SQLX.
type DriverType string

const (
	DriverMySQL    DriverType = "mysql"
	DriverPostgres DriverType = "postgres"
	DriverSQLite   DriverType = "sqlite3"
)

// ExtContext is an interface that unifies sqlx.DB and sqlx.Tx for contextual queries.
type ExtContext interface {
	sqlx.ExtContext
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
}

// Config holds the necessary configuration for an SQLX database connection.
type Config struct {
	Driver          DriverType
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	DebugMode       bool // If true, all queries are logged globally
}

// debugWrapper intercepts queries to log them before execution.
type debugWrapper struct {
	ExtContext
}

// logQuery prints the SQL query and its appended parameters.
func logQuery(ctx context.Context, method, query string, args []interface{}) {
	logw.CtxInfof(ctx, "[SQLX DEBUG] %s | Query: %s | Args: %v", method, query, args)
}

func (d *debugWrapper) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	logQuery(ctx, "Get", query, args)
	return d.ExtContext.GetContext(ctx, dest, query, args...)
}

func (d *debugWrapper) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	logQuery(ctx, "Select", query, args)
	return d.ExtContext.SelectContext(ctx, dest, query, args...)
}

func (d *debugWrapper) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	logQuery(ctx, "Exec", query, args)
	return d.ExtContext.ExecContext(ctx, query, args...)
}

// Connect establishes a connection using sqlx.
func Connect(ctx context.Context, cfg *Config) (*sqlx.DB, error) {
	db, err := sqlx.ConnectContext(ctx, string(cfg.Driver), cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("sqlxw: failed to connect: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	return db, nil
}

// Disconnect safely closes the SQLX database connection.
func Disconnect(db *sqlx.DB) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if db != nil {
			return db.Close()
		}
		return nil
	}
}

// Transaction starts a new database transaction and executes the provided closure.
func Transaction(ctx context.Context, db *sqlx.DB, fn func(txCtx context.Context) error) error {
	tx, err := db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	txCtx := context.WithValue(ctx, txKey, tx)
	if err = fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetDB extracts the SQLX transaction from the context if it exists.
// It applies a debug wrapper automatically if the global isDebug flag is true.
func GetDB(ctx context.Context, defaultDB *sqlx.DB, isDebug bool) ExtContext {
	var db ExtContext
	if tx, ok := ctx.Value(txKey).(*sqlx.Tx); ok {
		db = tx
	} else {
		db = defaultDB
	}

	if isDebug {
		return &debugWrapper{ExtContext: db}
	}
	return db
}

// Debug wraps an ExtContext to intercept and log its queries on-demand.
// This is useful for debugging specific queries without enabling global DebugMode.
// Usage: sqlxw.Debug(db).SelectContext(ctx, &dest, query, args...)
func Debug(db ExtContext) ExtContext {
	// Prevent double wrapping if it's already a debugWrapper
	if _, ok := db.(*debugWrapper); ok {
		return db
	}
	return &debugWrapper{ExtContext: db}
}
