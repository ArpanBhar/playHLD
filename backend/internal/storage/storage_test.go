package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrationsApplyAndRepeat(t *testing.T) {
	ctx := context.Background()
	pool, err := Open(ctx, filepath.Join(t.TempDir(), "migration-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("repeat migration failed: %v", err)
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='architecture_branches')`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("architecture_branches not created")
	}
}
