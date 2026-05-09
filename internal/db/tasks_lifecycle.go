package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MaxAttempts caps automatic re-runs. A task that reaches this many starts
// without a terminal success is marked failed instead of being claimed
// again. Catches the runaway loop where a task bug makes the worker crash,
// ResetStaleRunning recovers it on next start, the same crash repeats.
const MaxAttempts = 5

// NextPending claims the oldest pending row by flipping it to running atomically.
// Returns nil, nil when there is nothing to do.
//
// Tasks whose attempts are already at MaxAttempts are auto-failed in the
// same transaction (so they don't sit forever in pending) and skipped.
func (r *TaskRepo) NextPending() (*Task, error) {
	r.wmu.Lock()
	defer r.wmu.Unlock()

	for {
		tx, err := r.db.Begin()
		if err != nil {
			return nil, err
		}
		var id string
		var attempts int
		row := tx.QueryRow(
			`SELECT id, attempts FROM tasks WHERE status = ? ORDER BY created_at ASC LIMIT 1`,
			StatusPending,
		)
		if err := row.Scan(&id, &attempts); err != nil {
			_ = tx.Rollback()
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}
		// Cap reached - mark failed in this same tx and look for next one.
		if attempts >= MaxAttempts {
			finishedAt := time.Now().UTC()
			if _, err := tx.Exec(
				`UPDATE tasks
				 SET status = ?, error = ?, finished_at = ?
				 WHERE id = ?`,
				StatusFailed,
				fmt.Sprintf("aborted after %d attempts (cap reached)", attempts),
				finishedAt, id,
			); err != nil {
				_ = tx.Rollback()
				return nil, err
			}
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			continue // try the next pending row
		}

		now := time.Now().UTC()
		if _, err := tx.Exec(
			`UPDATE tasks SET status = ?, started_at = ?, attempts = attempts + 1 WHERE id = ?`,
			StatusRunning, now, id,
		); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		t, err := r.Get(id)
		if err != nil {
			return nil, err
		}
		return &t, nil
	}
}

// UpdateStatus changes status + optional error message; sets finished_at on terminal states.
func (r *TaskRepo) UpdateStatus(id string, status TaskStatus, errMsg string) error {
	r.wmu.Lock()
	defer r.wmu.Unlock()

	terminal := status == StatusSucceeded || status == StatusFailed || status == StatusCancelled
	var finishedAt sql.NullTime
	if terminal {
		finishedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	}
	var errVal sql.NullString
	if errMsg != "" {
		errVal = sql.NullString{String: errMsg, Valid: true}
	}
	_, err := r.db.Exec(
		`UPDATE tasks SET status = ?, error = COALESCE(?, error), finished_at = COALESCE(?, finished_at) WHERE id = ?`,
		string(status), errVal, finishedAt, id,
	)
	return err
}

// SaveMediaIDs records the media IDs returned by the API submit step.
func (r *TaskRepo) SaveMediaIDs(id string, ids []string) error {
	b, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	r.wmu.Lock()
	defer r.wmu.Unlock()
	_, err = r.db.Exec(`UPDATE tasks SET media_ids = ? WHERE id = ?`, string(b), id)
	return err
}

// AppendOutput pushes a single video path onto the task's video_paths JSON array.
//
// Done atomically inside the database via SQLite's json_insert() so a process
// crash between read and write cannot lose a path. Falls back to '[]' when
// the column is NULL on an older row.
func (r *TaskRepo) AppendOutput(id, videoPath string) error {
	r.wmu.Lock()
	defer r.wmu.Unlock()

	// json_insert(target, '$[#]', value) appends to the end of a JSON array.
	// The '#' index is SQLite-specific shorthand for "next array slot".
	_, err := r.db.Exec(
		`UPDATE tasks
		 SET video_paths = json_insert(COALESCE(video_paths, '[]'), '$[#]', ?)
		 WHERE id = ?`,
		videoPath, id,
	)
	return err
}

// Requeue clones an existing task as a new pending row.
func (r *TaskRepo) Requeue(id string) (Task, error) {
	src, err := r.Get(id)
	if err != nil {
		return Task{}, err
	}
	cfgJSON, err := json.Marshal(src.Config)
	if err != nil {
		return Task{}, err
	}
	t := Task{
		ID:           uuid.NewString(),
		Prompt:       src.Prompt,
		Config:       src.Config,
		Status:       StatusPending,
		CreatedAt:    time.Now().UTC(),
		SourceTaskID: src.ID,
	}
	r.wmu.Lock()
	defer r.wmu.Unlock()
	_, err = r.db.Exec(
		`INSERT INTO tasks (id, prompt, config_json, status, created_at, source_task_id)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		t.ID, t.Prompt, string(cfgJSON), string(t.Status), t.CreatedAt, t.SourceTaskID,
	)
	if err != nil {
		return Task{}, err
	}
	return t, nil
}

// Delete removes a task row. Caller is responsible for deleting any video files.
func (r *TaskRepo) Delete(id string) error {
	r.wmu.Lock()
	defer r.wmu.Unlock()
	_, err := r.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	return err
}

// ResetStaleRunning flips any rows stuck in 'running' (crash recovery on startup).
func (r *TaskRepo) ResetStaleRunning() (int, error) {
	r.wmu.Lock()
	defer r.wmu.Unlock()
	res, err := r.db.Exec(`UPDATE tasks SET status = ? WHERE status = ?`, StatusPending, StatusRunning)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
