package sqlxw

import (
	"context"
	"errors"
	"testing"

	"github.com/jmoiron/sqlx"
)

type TestUser struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

func setupTestDB(t *testing.T) *sqlx.DB {
	cfg := &Config{
		Driver: DriverSQLite,
		DSN:    ":memory:",
	}
	db, err := Connect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Create table
	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	return db
}

func TestSqlxTransaction_Commit(t *testing.T) {
	db := setupTestDB(t)
	defer Disconnect(db)(context.Background())

	ctx := context.Background()

	err := Transaction(ctx, db, func(txCtx context.Context) error {
		repoDB := GetDB(txCtx, db)
		_, err := repoDB.ExecContext(txCtx, "INSERT INTO users (id, name) VALUES (?, ?)", 1, "Alice")
		return err
	})

	if err != nil {
		t.Errorf("Transaction failed: %v", err)
	}

	var name string
	err = db.Get(&name, "SELECT name FROM users WHERE id = 1")
	if err != nil || name != "Alice" {
		t.Errorf("Expected Alice, got %v (err: %v)", name, err)
	}
}

func TestSqlxTransaction_Rollback(t *testing.T) {
	db := setupTestDB(t)
	defer Disconnect(db)(context.Background())

	ctx := context.Background()

	_ = Transaction(ctx, db, func(txCtx context.Context) error {
		repoDB := GetDB(txCtx, db)
		_, _ = repoDB.ExecContext(txCtx, "INSERT INTO users (id, name) VALUES (?, ?)", 2, "Bob")
		return errors.New("simulated error")
	})

	var count int
	db.Get(&count, "SELECT COUNT(*) FROM users WHERE id = 2")
	if count != 0 {
		t.Errorf("Expected 0 records after rollback, got %d", count)
	}
}
