package api

import (
	"context"
	"path/filepath"
	"testing"

	"playhld/backend/internal/storage"
)

func testDatabase(t *testing.T) *storage.DB {
	t.Helper()
	database, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "playhld-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := storage.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	return database
}
