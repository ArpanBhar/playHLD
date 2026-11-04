package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"modernc.org/sqlite"
	"playhld/backend/migrations"
)

// DB keeps the context-first database API used by the HTTP layer while using
// database/sql and SQLite underneath.
type DB struct{ *sql.DB }

type Tx struct{ *sql.Tx }

func (db *DB) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return db.QueryRowContext(ctx, query, args...)
}

func (db *DB) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.QueryContext(ctx, query, args...)
}

func (db *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.ExecContext(ctx, query, args...)
}

func (db *DB) Begin(ctx context.Context) (*Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{Tx: tx}, nil
}

func (db *DB) Ping(ctx context.Context) error { return db.PingContext(ctx) }

func (tx *Tx) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return tx.QueryRowContext(ctx, query, args...)
}

func (tx *Tx) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.ExecContext(ctx, query, args...)
}

func (tx *Tx) Commit(context.Context) error   { return tx.Tx.Commit() }
func (tx *Tx) Rollback(context.Context) error { return tx.Tx.Rollback() }

func sqliteDSN(path string) (string, error) {
	if path == ":memory:" {
		return "file:playhld-memory?mode=memory&cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_time_format=sqlite", nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
		return "", err
	}
	urlPath := filepath.ToSlash(absolute)
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}
	fileURL := &url.URL{Scheme: "file", Path: urlPath}
	query := fileURL.Query()
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "journal_mode(WAL)")
	query.Set("_time_format", "sqlite")
	fileURL.RawQuery = query.Encode()
	return fileURL.String(), nil
}

func Open(ctx context.Context, path string) (*DB, error) {
	dsn, err := sqliteDSN(path)
	if err != nil {
		return nil, err
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// A single connection gives predictable transaction boundaries and also
	// keeps in-memory databases consistent across every request.
	database.SetMaxOpenConns(1)
	if err := database.PingContext(ctx); err != nil {
		database.Close()
		return nil, err
	}
	return &DB{DB: database}, nil
}

func IsUniqueConstraint(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		// SQLITE_CONSTRAINT_PRIMARYKEY and SQLITE_CONSTRAINT_UNIQUE.
		return sqliteErr.Code() == 1555 || sqliteErr.Code() == 2067
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func Migrate(ctx context.Context, database *DB) error {
	if _, err := database.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return err
	}
	entries, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries)
	for _, name := range entries {
		var applied bool
		if err := database.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)`, name).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := database.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(body)); err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO schema_migrations(name) VALUES($1)`, name)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
