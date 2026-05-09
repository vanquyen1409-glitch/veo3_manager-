package main

import (
	"fmt"

	"veo3-manager/internal/db"
)

// EnqueuePrompts inserts each non-empty prompt as a pending task.
func (a *App) EnqueuePrompts(prompts []string, cfg db.GenerationConfig) ([]db.Task, error) {
	if a.tasks == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = a.outputDir
	}
	if cfg.OutputCount < 1 {
		cfg.OutputCount = 1
	}
	if cfg.OutputCount > 4 {
		cfg.OutputCount = 4
	}
	return a.tasks.BatchEnqueue(prompts, cfg)
}

// ListTasks returns rows for the Queue / History views.
func (a *App) ListTasks(filter db.ListFilter) ([]db.Task, error) {
	if a.tasks == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	out, err := a.tasks.List(filter)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []db.Task{}
	}
	return out, nil
}

// GetTask returns one task by id.
func (a *App) GetTask(id string) (db.Task, error) {
	if a.tasks == nil {
		return db.Task{}, fmt.Errorf("database not initialized")
	}
	return a.tasks.Get(id)
}

// RequeueTask clones a finished/failed task back into the pending queue.
func (a *App) RequeueTask(id string) (db.Task, error) {
	if a.tasks == nil {
		return db.Task{}, fmt.Errorf("database not initialized")
	}
	return a.tasks.Requeue(id)
}

// DeleteTask removes a task row (does not delete output files).
func (a *App) DeleteTask(id string) error {
	if a.tasks == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.tasks.Delete(id)
}

// GetStats returns the dashboard summary.
func (a *App) GetStats() (db.Stats, error) {
	if a.tasks == nil {
		return db.Stats{}, fmt.Errorf("database not initialized")
	}
	return a.tasks.Stats()
}
