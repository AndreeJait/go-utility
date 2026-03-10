package migratew

import (
	"database/sql"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/mattn/go-sqlite3" // Required to open the SQLite memory connection
)

// setupTestFS creates an in-memory virtual filesystem loaded with mock SQL migration files.
func setupTestFS() fstest.MapFS {
	return fstest.MapFS{
		"migrations/000001_create_users.up.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);`),
		},
		"migrations/000001_create_users.down.sql": &fstest.MapFile{
			Data: []byte(`DROP TABLE users;`),
		},
		"migrations/000002_add_email.up.sql": &fstest.MapFile{
			Data: []byte(`ALTER TABLE users ADD COLUMN email TEXT;`),
		},
		"migrations/000002_add_email.down.sql": &fstest.MapFile{
			// SQLite requires table recreation to drop columns, handled safely here for testing
			Data: []byte(`
			CREATE TABLE users_temp (id INTEGER PRIMARY KEY, name TEXT);
			DROP TABLE users;
			ALTER TABLE users_temp RENAME TO users;
			`),
		},
	}
}

// TestMigrator_Lifecycle validates the end-to-end flow of Up, Steps, and Down migrations.
func TestMigrator_Lifecycle(t *testing.T) {
	// 1. Open an isolated, in-memory SQLite database connection
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Failed to open sqlite memory db: %v", err)
	}
	defer db.Close()

	// 2. Initialize the virtual file system
	mockFS := setupTestFS()

	// 3. Initialize the Migrator engine
	m, err := New(db, SQLite, mockFS, "migrations")
	if err != nil {
		t.Fatalf("Failed to initialize migrator: %v", err)
	}

	// TEST A: Run Up() to apply all migrations
	t.Run("Up", func(t *testing.T) {
		if err := m.Up(); err != nil {
			t.Fatalf("Expected Up() to succeed, got: %v", err)
		}

		// Verify the database reached version 2 cleanly
		version, dirty, err := m.Version()
		if err != nil {
			t.Fatalf("Failed to retrieve version: %v", err)
		}
		if dirty {
			t.Errorf("Expected database state to be clean, but it is dirty")
		}
		if version != 2 {
			t.Errorf("Expected migration version 2, got %d", version)
		}
	})

	// TEST B: Rollback by 1 step using Steps()
	t.Run("Steps_Down", func(t *testing.T) {
		if err := m.Steps(-1); err != nil {
			t.Fatalf("Expected Steps(-1) to succeed, got: %v", err)
		}

		// Verify the database rolled back to version 1
		version, _, err := m.Version()
		if err != nil {
			t.Fatalf("Failed to retrieve version: %v", err)
		}
		if version != 1 {
			t.Errorf("Expected migration version 1 after stepping down, got %d", version)
		}
	})

	// TEST C: Run Down() to rollback all remaining migrations
	t.Run("Down", func(t *testing.T) {
		if err := m.Down(); err != nil {
			t.Fatalf("Expected Down() to succeed, got: %v", err)
		}

		// Verify the database version is completely empty (ErrNilVersion)
		_, _, err = m.Version()
		if !errors.Is(err, migrate.ErrNilVersion) {
			t.Errorf("Expected ErrNilVersion after a full Down() rollback, got: %v", err)
		}
	})
}

// TestMigrator_Options verifies that functional configuration options are applied correctly.
func TestMigrator_Options(t *testing.T) {
	cfg := &Config{}
	opt := WithSchema("tenant_auth_schema")
	opt(cfg)

	if cfg.SchemaName != "tenant_auth_schema" {
		t.Errorf("Expected SchemaName to be 'tenant_auth_schema', got '%s'", cfg.SchemaName)
	}
}
