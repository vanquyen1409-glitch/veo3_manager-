package db

import (
	"fmt"
	"strings"
)

// Get fetches a task by id.
func (r *TaskRepo) Get(id string) (Task, error) {
	row := r.db.QueryRow(
		`SELECT id, prompt, config_json, status, COALESCE(error,''),
		        COALESCE(media_ids,''), COALESCE(video_paths,''), COALESCE(thumb_paths,''),
		        attempts, created_at, started_at, finished_at, COALESCE(source_task_id,'')
		 FROM tasks WHERE id = ?`, id,
	)
	return scanTask(row)
}

// List returns rows matching filter; defaults to ORDER BY created_at DESC LIMIT 50.
func (r *TaskRepo) List(f ListFilter) ([]Task, error) {
	var (
		clauses []string
		args    []any
	)
	if len(f.Statuses) > 0 {
		ph := make([]string, 0, len(f.Statuses))
		for _, s := range f.Statuses {
			ph = append(ph, "?")
			args = append(args, string(s))
		}
		clauses = append(clauses, fmt.Sprintf("status IN (%s)", strings.Join(ph, ",")))
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		clauses = append(clauses, "prompt LIKE ?")
		args = append(args, "%"+s+"%")
	}
	if f.From != nil {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, f.From.UTC())
	}
	if f.To != nil {
		clauses = append(clauses, "created_at <= ?")
		args = append(args, f.To.UTC())
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}
	// orderBy/dir come through a strict allowlist - if any future caller passes
	// a value outside these maps it falls back to the default. This is the
	// belt-and-braces guard for ORDER BY (cannot be parameterised in SQL).
	allowedOrderBy := map[string]string{
		"created_at":  "created_at",
		"finished_at": "finished_at",
	}
	orderBy, ok := allowedOrderBy[f.OrderBy]
	if !ok {
		orderBy = "created_at"
	}
	allowedDir := map[string]string{
		"asc":  "ASC",
		"ASC":  "ASC",
		"desc": "DESC",
		"DESC": "DESC",
	}
	dir, ok := allowedDir[f.OrderDir]
	if !ok {
		dir = "DESC"
	}

	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	// LIMIT and OFFSET are now parameterised (?) so they can never be string-
	// concatenated - even though they were ints, this is the cheaper-than-
	// auditing-forever pattern.
	q := fmt.Sprintf(
		`SELECT id, prompt, config_json, status, COALESCE(error,''),
		        COALESCE(media_ids,''), COALESCE(video_paths,''), COALESCE(thumb_paths,''),
		        attempts, created_at, started_at, finished_at, COALESCE(source_task_id,'')
		 FROM tasks %s ORDER BY %s %s LIMIT ? OFFSET ?`,
		where, orderBy, dir,
	)
	args = append(args, limit, offset)
	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
