package migration

import (
	"testing"

	"github.com/alireza0/s-ui/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}
	return db
}

func TestTo1_4_CreatesTables(t *testing.T) {
	db := setupMigrationDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	err := to1_4(tx)
	if err != nil {
		t.Fatalf("to1_4 failed: %v", err)
	}

	// Verify client_autoresets table exists
	if !tx.Migrator().HasTable(&model.ClientAutoreset{}) {
		t.Error("expected client_autoresets table to exist")
	}

	// Verify traffic_histories table exists
	if !tx.Migrator().HasTable(&model.TrafficHistory{}) {
		t.Error("expected traffic_histories table to exist")
	}
}

func TestTo1_4_Idempotent(t *testing.T) {
	db := setupMigrationDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	// Run twice
	if err := to1_4(tx); err != nil {
		t.Fatalf("first call to to1_4 failed: %v", err)
	}
	if err := to1_4(tx); err != nil {
		t.Fatalf("second call to to1_4 failed (not idempotent): %v", err)
	}
}
