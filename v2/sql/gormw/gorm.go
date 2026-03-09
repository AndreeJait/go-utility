package gormw

import (
	"context"
	"fmt"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type contextKey string

const txKey contextKey = "gormw-tx"

// DriverType defines the supported SQL databases for GORM.
type DriverType string

const (
	DriverMySQL    DriverType = "mysql"
	DriverPostgres DriverType = "postgres"
	DriverSQLite   DriverType = "sqlite"
)

// Config holds the necessary configuration for a GORM database connection.
type Config struct {
	Driver          DriverType
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	DebugMode       bool         // If true, enables global SQL logging
	GormConfig      *gorm.Config // Optional: custom GORM config
}

// customLogger bridges GORM's internal logging with our standard logw package.
type customLogger struct {
	LogLevel logger.LogLevel
}

func (l *customLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l *customLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		logw.CtxInfof(ctx, msg, data...)
	}
}

func (l *customLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		logw.CtxWarningf(ctx, msg, data...)
	}
}

func (l *customLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		logw.CtxErrorf(ctx, msg, data...)
	}
}

func (l *customLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}
	elapsed := time.Since(begin)
	sql, rows := fc() // GORM automatically interpolates parameters here!

	if err != nil {
		logw.CtxErrorf(ctx, "[GORM] %v | %s | rows: %d | err: %v", elapsed, sql, rows, err)
	} else if l.LogLevel >= logger.Info {
		logw.CtxInfof(ctx, "[GORM] %v | %s | rows: %d", elapsed, sql, rows)
	}
}

// Connect establishes a connection to the database using GORM.
func Connect(ctx context.Context, cfg *Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case DriverMySQL:
		dialector = mysql.Open(cfg.DSN)
	case DriverPostgres:
		dialector = postgres.Open(cfg.DSN)
	case DriverSQLite:
		dialector = sqlite.Open(cfg.DSN)
	default:
		return nil, fmt.Errorf("gormw: unsupported driver %s", cfg.Driver)
	}

	gConfig := cfg.GormConfig
	if gConfig == nil {
		gConfig = &gorm.Config{}
	}

	// Attach our custom logw bridge
	logLevel := logger.Warn
	if cfg.DebugMode {
		logLevel = logger.Info
	}
	gConfig.Logger = &customLogger{LogLevel: logLevel}

	db, err := gorm.Open(dialector, gConfig)
	if err != nil {
		return nil, fmt.Errorf("gormw: failed to connect: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("gormw: failed to get sql.DB: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	return db, nil
}

// Disconnect safely closes the underlying sql.DB connection.
func Disconnect(db *gorm.DB) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if db != nil {
			if sqlDB, err := db.DB(); err == nil {
				return sqlDB.Close()
			}
		}
		return nil
	}
}

// Transaction executes a function within a database transaction.
func Transaction(ctx context.Context, db *gorm.DB, fn func(txCtx context.Context) error) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey, tx)
		return fn(txCtx)
	})
}

// GetDB extracts the GORM transaction from the context if it exists.
// If it doesn't exist, it returns the default DB.
func GetDB(ctx context.Context, defaultDB *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return defaultDB.WithContext(ctx)
}

// Debug returns a new GORM session with debug mode enabled.
// It will print the interpolated SQL and parameters for the chained query.
// Usage: gormw.Debug(db).Where("id = ?", 1).First(&user)
func Debug(db *gorm.DB) *gorm.DB {
	return db.Debug()
}
