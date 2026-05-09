package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// CheckpointOnClose runs `PRAGMA wal_checkpoint(TRUNCATE)` to fold the WAL
// file back into the main DB and shrink it to zero on exit. Without this,
// long-running sessions can leave a multi-hundred-MB queue.db-wal sidecar
// that never gets reclaimed because SQLite only checkpoints opportunistically.
//
// Errors are logged-only (not returned) - this is best-effort cleanup, the
// data is safe regardless.
func CheckpointOnClose(db *sql.DB) {
	if db == nil {
		return
	}
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		slog.Warn("wal_checkpoint failed", "err", err)
	}
}

// Open initializes the SQLite database at <appDataDir>/queue.db, applies
// pragmas, and runs idempotent migrations. Caller owns the *sql.DB.
func Open(appDataDir string) (*sql.DB, error) {
	if err := os.MkdirAll(appDataDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir app data: %w", err)
	}
	dbPath := filepath.Join(appDataDir, "queue.db")
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// modernc/sqlite is single-writer; cap connections to keep semantics simple.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}
