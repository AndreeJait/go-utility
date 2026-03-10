// Package migratew provides a robust, framework-agnostic database migration utility.
// It acts as a streamlined wrapper around golang-migrate, natively supporting
// embedded SQL files (go:embed) and standard *sql.DB connections.
package migratew

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/AndreeJait/go-utility/v2/logw" // Adjust to your actual module path
)

// DriverName represents the supported SQL database drivers.
type DriverName string

const (
	Postgres DriverName = "postgres"
	MySQL    DriverName = "mysql"
	SQLite   DriverName = "sqlite3"
)

// Migrator defines the contract for executing database schema changes safely.
type Migrator interface {
	// Up applies all pending migrations to the database.
	// It safely ignores migrate.ErrNoChange if the database is already up to date.
	Up() error

	// Down rolls back all applied migrations by executing the .down.sql files.
	// WARNING: This will destroy table structures and data. Use with extreme caution.
	Down() error

	// Steps migrates the database up or down by a specific number of steps.
	// A positive 'n' executes Up migrations, while a negative 'n' executes Down migrations.
	Steps(n int) error

	// Version returns the currently active migration version and a boolean indicating
	// if the database schema is in a "dirty" (failed/inconsistent) state.
	Version() (uint, bool, error)

	// Close safely terminates the underlying migration engine and releases resources.
	// It does not close the underlying *sql.DB connection.
	Close() error
}

type migrator struct {
	engine *migrate.Migrate
}

// Config holds optional configurations for the migrator engine.
type Config struct {
	SchemaName string // Target schema for PostgreSQL (defaults to "public" if empty)
}

// Option applies a configuration modifier to the Migrator.
type Option func(*Config)

// WithSchema sets a specific database schema to run migrations against.
// This is primarily utilized by PostgreSQL for multi-tenant architectures.
func WithSchema(schema string) Option {
	return func(c *Config) {
		c.SchemaName = schema
	}
}

// New initializes a new Migrator instance.
// It requires a standard *sql.DB instance, the target driver name, an embedded filesystem (fs.FS),
// the internal directory path containing the .sql files, and any optional configurations.
func New(db *sql.DB, driver DriverName, migrationFS fs.FS, dirPath string, opts ...Option) (Migrator, error) {
	cfg := &Config{}
	for _, opt := range opts {
		opt(cfg)
	}

	// 1. Setup the embedded filesystem source
	sourceDriver, err := iofs.New(migrationFS, dirPath)
	if err != nil {
		return nil, fmt.Errorf("migratew: failed to create iofs driver: %w", err)
	}

	// 2. Setup the target database driver based on the provided DriverName
	var dbDriver database.Driver
	switch driver {
	case Postgres:
		pgConfig := &postgres.Config{}
		if cfg.SchemaName != "" {
			pgConfig.SchemaName = cfg.SchemaName
		}
		dbDriver, err = postgres.WithInstance(db, pgConfig)
	case MySQL:
		dbDriver, err = mysql.WithInstance(db, &mysql.Config{})
	case SQLite:
		dbDriver, err = sqlite3.WithInstance(db, &sqlite3.Config{})
	default:
		return nil, fmt.Errorf("migratew: unsupported database driver: %s", driver)
	}

	if err != nil {
		return nil, fmt.Errorf("migratew: failed to initialize db driver: %w", err)
	}

	// 3. Create the golang-migrate engine instance
	engine, err := migrate.NewWithInstance(
		"iofs", sourceDriver,
		string(driver), dbDriver,
	)
	if err != nil {
		return nil, fmt.Errorf("migratew: failed to create migration engine: %w", err)
	}

	return &migrator{engine: engine}, nil
}

func (m *migrator) Up() error {
	logw.Info("migratew: running Up migrations...")
	err := m.engine.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logw.Info("migratew: database is already up to date. No changes applied.")
			return nil
		}
		return fmt.Errorf("migratew: failed to apply up migrations: %w", err)
	}
	logw.Info("migratew: successfully applied all up migrations.")
	return nil
}

func (m *migrator) Down() error {
	logw.Warning("migratew: running Down migrations (rolling back all!)...")
	err := m.engine.Down()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return fmt.Errorf("migratew: failed to apply down migrations: %w", err)
	}
	logw.Info("migratew: successfully rolled back all migrations.")
	return nil
}

func (m *migrator) Steps(n int) error {
	logw.Infof("migratew: running migrations by %d steps...", n)
	err := m.engine.Steps(n)
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migratew: failed to execute steps: %w", err)
	}
	return nil
}

func (m *migrator) Version() (uint, bool, error) {
	return m.engine.Version()
}

func (m *migrator) Close() error {
	sourceErr, dbErr := m.engine.Close()
	if sourceErr != nil {
		return fmt.Errorf("migratew: source close error: %w", sourceErr)
	}
	if dbErr != nil {
		return fmt.Errorf("migratew: db close error: %w", dbErr)
	}
	return nil
}
