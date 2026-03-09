package gormw

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"
)

type TestUser struct {
	ID   int `gorm:"primaryKey"`
	Name string
}

func setupTestDB(t *testing.T) *gorm.DB {
	cfg := &Config{
		Driver: DriverSQLite,
		DSN:    "file::memory:?cache=shared", // In-memory database
	}
	db, err := Connect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Failed to connect to test db: %v", err)
	}
	db.AutoMigrate(&TestUser{})
	return db
}

func TestGormTransaction_Commit(t *testing.T) {
	db := setupTestDB(t)
	defer Disconnect(db)(context.Background())

	ctx := context.Background()

	err := Transaction(ctx, db, func(txCtx context.Context) error {
		// Simulate Repository call using GetDB
		repoDB := GetDB(txCtx, db)
		return repoDB.Create(&TestUser{ID: 1, Name: "Alice"}).Error
	})

	if err != nil {
		t.Errorf("Transaction failed: %v", err)
	}

	var count int64
	db.Model(&TestUser{}).Count(&count)
	if count != 1 {
		t.Errorf("Expected 1 record after commit, got %d", count)
	}
}

func TestGormTransaction_Rollback(t *testing.T) {
	db := setupTestDB(t)
	defer Disconnect(db)(context.Background())

	ctx := context.Background()

	_ = Transaction(ctx, db, func(txCtx context.Context) error {
		repoDB := GetDB(txCtx, db)
		repoDB.Create(&TestUser{ID: 2, Name: "Bob"})

		// Simulate an error in usecase
		return errors.New("something went wrong, abort!")
	})

	var count int64
	db.Model(&TestUser{}).Where("id = ?", 2).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 records after rollback, got %d", count)
	}
}
