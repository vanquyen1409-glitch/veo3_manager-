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
	orderBy := "created_at"
	if f.OrderBy == "finished_at" {
		orderBy = "finished_at"
	}
	dir := "DESC"
	if strings.EqualFold(f.OrderDir, "asc") {
		dir = "ASC"
	}
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	q := fmt.Sprintf(
		`SELECT id, prompt, config_json, status, COALESCE(error,''),
		        COALESCE(media_ids,''), COALESCE(video_paths,''), COALESCE(thumb_paths,''),
		        attempts, created_at, started_at, finished_at, COALESCE(source_task_id,'')
		 FROM tasks %s ORDER BY %s %s LIMIT %d OFFSET %d`,
		where, orderBy, dir, limit, offset,
	)
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
