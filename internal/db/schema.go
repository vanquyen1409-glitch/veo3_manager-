package db

import (
	"database/sql"
	"fmt"
)

type migration struct {
	version int
	stmt    string
}

// migrations are applied in order. Add new entries; never edit an existing entry.
var migrations = []migration{
	{
		version: 1,
		stmt: `
CREATE TABLE IF NOT EXISTS tasks (
  id              TEXT PRIMARY KEY,
  prompt          TEXT NOT NULL,
  config_json     TEXT NOT NULL,
  status          TEXT NOT NULL,
  error           TEXT,
  media_ids       TEXT,
  video_paths     TEXT,
  thumb_paths     TEXT,
  attempts        INTEGER NOT NULL DEFAULT 0,
  created_at      DATETIME NOT NULL,
  started_at      DATETIME,
  finished_at     DATETIME,
  source_task_id  TEXT
);
CREATE INDEX IF NOT EXISTS idx_tasks_status     ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC);

CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`,
	},
}

func migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY)`); err != nil {
		return err
	}
	var current int
	if err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&current); err != nil {
		return err
	}
	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(m.stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration v%d: %w", m.version, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_version(version) VALUES (?)`, m.version); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
