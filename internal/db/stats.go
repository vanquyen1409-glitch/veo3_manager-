package db

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Stats computes the dashboard summary.
func (r *TaskRepo) Stats() (Stats, error) {
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	weekStart := dayStart.AddDate(0, 0, -6) // last 7 days inclusive

	today, err := r.statBucket(dayStart)
	if err != nil {
		return Stats{}, err
	}
	week, err := r.statBucket(weekStart)
	if err != nil {
		return Stats{}, err
	}
	allTime, err := r.statBucket(time.Time{})
	if err != nil {
		return Stats{}, err
	}

	var pending, running int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = ?`, StatusPending).Scan(&pending); err != nil {
		return Stats{}, err
	}
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = ?`, StatusRunning).Scan(&running); err != nil {
		return Stats{}, err
	}

	dailyOut, err := r.dailyCounts(dayStart)
	if err != nil {
		return Stats{}, err
	}

	avgMs, err := r.avgSucceededDurationMs()
	if err != nil {
		return Stats{}, err
	}

	totalVideos := r.countDownloadedVideos()

	return Stats{
		Today:         today,
		Last7Days:     week,
		AllTime:       allTime,
		PendingCount:  pending,
		RunningCount:  running,
		DailyCounts:   dailyOut,
		AvgDurationMs: avgMs,
		TotalVideos:   totalVideos,
	}, nil
}

func (r *TaskRepo) statBucket(from time.Time) (StatBucket, error) {
	var b StatBucket
	row := r.db.QueryRow(
		`SELECT
		  COUNT(*),
		  SUM(CASE WHEN status = ? THEN 1 ELSE 0 END),
		  SUM(CASE WHEN status = ? THEN 1 ELSE 0 END)
		 FROM tasks WHERE created_at >= ?`,
		StatusSucceeded, StatusFailed, from,
	)
	var total, ok, fail sql.NullInt64
	if err := row.Scan(&total, &ok, &fail); err != nil {
		return b, err
	}
	b.Total = int(total.Int64)
	b.Succeeded = int(ok.Int64)
	b.Failed = int(fail.Int64)
	return b, nil
}

// dailyCounts returns the last 14 days of succeeded/failed counts, anchored
// at dayStart. Missing days are emitted with zeroes so the chart has 14 bars.
func (r *TaskRepo) dailyCounts(dayStart time.Time) ([]DailyCount, error) {
	from14 := dayStart.AddDate(0, 0, -13)
	rows, err := r.db.Query(
		`SELECT date(created_at),
		        SUM(CASE WHEN status = ? THEN 1 ELSE 0 END),
		        SUM(CASE WHEN status = ? THEN 1 ELSE 0 END)
		 FROM tasks WHERE created_at >= ? GROUP BY date(created_at)`,
		StatusSucceeded, StatusFailed, from14,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	daily := map[string]DailyCount{}
	for rows.Next() {
		var d string
		var ok, fail sql.NullInt64
		if err := rows.Scan(&d, &ok, &fail); err != nil {
			return nil, err
		}
		daily[d] = DailyCount{Date: d, Succeeded: int(ok.Int64), Failed: int(fail.Int64)}
	}

	out := make([]DailyCount, 0, 14)
	for i := 13; i >= 0; i-- {
		d := dayStart.AddDate(0, 0, -i).Format("2006-01-02")
		if v, ok := daily[d]; ok {
			out = append(out, v)
			continue
		}
		out = append(out, DailyCount{Date: d})
	}
	return out, nil
}

func (r *TaskRepo) avgSucceededDurationMs() (int64, error) {
	var avgMs sql.NullFloat64
	if err := r.db.QueryRow(
		`SELECT AVG((julianday(finished_at) - julianday(started_at)) * 86400000)
		 FROM tasks WHERE status = ? AND started_at IS NOT NULL AND finished_at IS NOT NULL`,
		StatusSucceeded,
	).Scan(&avgMs); err != nil {
		return 0, err
	}
	return int64(avgMs.Float64), nil
}

func (r *TaskRepo) countDownloadedVideos() int {
	rows, err := r.db.Query(`SELECT video_paths FROM tasks WHERE video_paths IS NOT NULL AND video_paths != ''`)
	if err != nil {
		return 0
	}
	defer rows.Close()
	total := 0
	for rows.Next() {
		var s string
		if rows.Scan(&s) != nil || s == "" {
			continue
		}
		var arr []string
		if json.Unmarshal([]byte(s), &arr) == nil {
			total += len(arr)
		}
	}
	return total
}
