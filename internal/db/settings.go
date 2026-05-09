package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
)

// SettingsRepo persists key/value settings as JSON-encoded strings. The
// frontend reads and writes the whole bundle for simplicity. The bundle is
// cached in-memory so hot paths (browser cfgFn, worker loop) don't hit
// SQLite on every call. SetBundle invalidates the cache.
type SettingsRepo struct {
	db          *sql.DB
	mu          sync.Mutex
	defaultDirs DefaultDirs

	cacheMu sync.RWMutex
	cache   *SettingsBundle
}

// DefaultDirs supplies fallback paths when settings are unset on first run.
type DefaultDirs struct {
	UserDataDir string
	OutputDir   string
}

func NewSettingsRepo(db *sql.DB, def DefaultDirs) *SettingsRepo {
	return &SettingsRepo{db: db, defaultDirs: def}
}

// invalidateCache drops the cached bundle. Call from any path that mutates
// settings rows.
func (r *SettingsRepo) invalidateCache() {
	r.cacheMu.Lock()
	r.cache = nil
	r.cacheMu.Unlock()
}

// Get returns the raw stored value or "" when missing.
func (r *SettingsRepo) Get(key string) (string, error) {
	var v string
	err := r.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// Set stores a raw string value. Invalidates the cache so the next GetBundle
// re-reads from disk.
func (r *SettingsRepo) Set(key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	defer r.invalidateCache()
	_, err := r.db.Exec(
		`INSERT INTO settings(key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

// GetBundle returns the full SettingsBundle, applying built-in defaults
// for any missing keys so the frontend always sees populated values. The
// result is cached until the next SetBundle call.
func (r *SettingsRepo) GetBundle() (SettingsBundle, error) {
	r.cacheMu.RLock()
	if r.cache != nil {
		out := *r.cache
		r.cacheMu.RUnlock()
		return out, nil
	}
	r.cacheMu.RUnlock()

	bundle, err := r.loadBundle()
	if err != nil {
		return bundle, err
	}
	r.cacheMu.Lock()
	c := bundle
	r.cache = &c
	r.cacheMu.Unlock()
	return bundle, nil
}

func (r *SettingsRepo) loadBundle() (SettingsBundle, error) {
	b := SettingsBundle{
		ChromePath:      "",
		UserDataDir:     r.defaultDirs.UserDataDir,
		OutputDir:       r.defaultDirs.OutputDir,
		ChromeDebugPort: 9222,
		DefaultConfig: GenerationConfig{
			Model:       "veo_3_1_t2v_fast",
			AspectRatio: "16:9",
			OutputCount: 1,
			SeedBase:    0,
			OutputDir:   r.defaultDirs.OutputDir,
		},
		PollIntervalMs: 10000,
		PollTimeoutMs:  300000,
		MaxParallelDl:  1,
	}

	rows, err := r.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return b, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return b, err
		}
		applyKV(&b, k, v)
	}
	if b.DefaultConfig.OutputDir == "" {
		b.DefaultConfig.OutputDir = b.OutputDir
	}
	// Coerce the deprecated `veo_3_1_t2v_fast_ultra` model key to the actual
	// working model id discovered via labs.google API trace. Old settings
	// rows from earlier alpha builds may still hold the wrong value.
	if b.DefaultConfig.Model == "veo_3_1_t2v_fast_ultra" || b.DefaultConfig.Model == "" {
		b.DefaultConfig.Model = "veo_3_1_t2v_fast"
	}
	// Veo only ships landscape + portrait. If a stored config asks for 1:1
	// (image-mode aspect), fall back to landscape.
	if b.DefaultConfig.AspectRatio != "16:9" && b.DefaultConfig.AspectRatio != "9:16" {
		b.DefaultConfig.AspectRatio = "16:9"
	}
	// Normalize paths.
	b.UserDataDir = filepath.Clean(b.UserDataDir)
	b.OutputDir = filepath.Clean(b.OutputDir)
	return b, rows.Err()
}

// SetBundle persists every field of the bundle. Atomic via single transaction.
// Invalidates the GetBundle cache on success so subsequent reads see the new
// values immediately.
func (r *SettingsRepo) SetBundle(b SettingsBundle) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	defer r.invalidateCache()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(
		`INSERT INTO settings(key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
	)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	cfgJSON, err := json.Marshal(b.DefaultConfig)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	pairs := []struct{ k, v string }{
		{"chromePath", b.ChromePath},
		{"userDataDir", b.UserDataDir},
		{"outputDir", b.OutputDir},
		{"chromeDebugPort", jsonInt(b.ChromeDebugPort)},
		{"defaultConfig", string(cfgJSON)},
		{"pollIntervalMs", jsonInt(b.PollIntervalMs)},
		{"pollTimeoutMs", jsonInt(b.PollTimeoutMs)},
		{"maxParallelDl", jsonInt(b.MaxParallelDl)},
	}
	for _, p := range pairs {
		if _, err := stmt.Exec(p.k, p.v); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func applyKV(b *SettingsBundle, k, v string) {
	switch k {
	case "chromePath":
		b.ChromePath = v
	case "userDataDir":
		if v != "" {
			b.UserDataDir = v
		}
	case "outputDir":
		if v != "" {
			b.OutputDir = v
		}
	case "chromeDebugPort":
		if i, ok := parseInt(v); ok && i > 0 {
			b.ChromeDebugPort = i
		}
	case "defaultConfig":
		var cfg GenerationConfig
		if err := json.Unmarshal([]byte(v), &cfg); err == nil {
			b.DefaultConfig = cfg
		}
	case "pollIntervalMs":
		if i, ok := parseInt(v); ok && i > 0 {
			b.PollIntervalMs = i
		}
	case "pollTimeoutMs":
		if i, ok := parseInt(v); ok && i > 0 {
			b.PollTimeoutMs = i
		}
	case "maxParallelDl":
		if i, ok := parseInt(v); ok && i > 0 {
			b.MaxParallelDl = i
		}
	}
}

func jsonInt(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func parseInt(s string) (int, bool) {
	var i int
	if err := json.Unmarshal([]byte(s), &i); err != nil {
		return 0, false
	}
	return i, true
}
