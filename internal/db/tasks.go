package db

import (
	"database/sql"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskRepo serializes writes through wmu so concurrent callers (worker +
// frontend buttons) cannot interleave updates. Reads run without holding
// the lock since SQLite WAL allows concurrent readers.
type TaskRepo struct {
	db  *sql.DB
	wmu sync.Mutex
}

func NewTaskRepo(db *sql.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

// Enqueue persists one Task with status=pending.
func (r *TaskRepo) Enqueue(prompt string, cfg GenerationConfig) (Task, error) {
	t := Task{
		ID:        uuid.NewString(),
		Prompt:    prompt,
		Config:    cfg,
		Status:    StatusPending,
		CreatedAt: time.Now().UTC(),
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return Task{}, err
	}
	r.wmu.Lock()
	defer r.wmu.Unlock()
	_, err = r.db.Exec(
		`INSERT INTO tasks (id, prompt, config_json, status, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		t.ID, t.Prompt, string(cfgJSON), string(t.Status), t.CreatedAt,
	)
	if err != nil {
		return Task{}, err
	}
	return t, nil
}

// BatchEnqueue inserts many tasks in a single transaction.
func (r *TaskRepo) BatchEnqueue(prompts []string, cfg GenerationConfig) ([]Task, error) {
	if len(prompts) == 0 {
		return nil, nil
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	r.wmu.Lock()
	defer r.wmu.Unlock()

	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	stmt, err := tx.Prepare(`INSERT INTO tasks (id, prompt, config_json, status, created_at) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	defer stmt.Close()

	out := make([]Task, 0, len(prompts))
	for _, prompt := range prompts {
		prompt = strings.TrimSpace(prompt)
		if prompt == "" {
			continue
		}
		t := Task{ID: uuid.NewString(), Prompt: prompt, Config: cfg, Status: StatusPending, CreatedAt: now}
		if _, err := stmt.Exec(t.ID, t.Prompt, string(cfgJSON), string(t.Status), t.CreatedAt); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		out = append(out, t)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(s rowScanner) (Task, error) {
	var (
		t          Task
		cfgJSON    string
		mediaIDs   string
		videoPaths string
		thumbPaths string
		startedAt  sql.NullTime
		finishedAt sql.NullTime
	)
	if err := s.Scan(
		&t.ID, &t.Prompt, &cfgJSON, &t.Status, &t.Error,
		&mediaIDs, &videoPaths, &thumbPaths,
		&t.Attempts, &t.CreatedAt, &startedAt, &finishedAt, &t.SourceTaskID,
	); err != nil {
		return Task{}, err
	}
	if cfgJSON != "" {
		_ = json.Unmarshal([]byte(cfgJSON), &t.Config)
	}
	if mediaIDs != "" {
		_ = json.Unmarshal([]byte(mediaIDs), &t.MediaIDs)
	}
	if videoPaths != "" {
		_ = json.Unmarshal([]byte(videoPaths), &t.VideoPaths)
	}
	if thumbPaths != "" {
		_ = json.Unmarshal([]byte(thumbPaths), &t.ThumbPaths)
	}
	if startedAt.Valid {
		v := startedAt.Time
		t.StartedAt = &v
	}
	if finishedAt.Valid {
		v := finishedAt.Time
		t.FinishedAt = &v
	}
	return t, nil
}
