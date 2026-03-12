package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"tinybot/internal/domain/model"
)

// FileCronRepository 实现了 CronRepository 接口，使用 JSON 文件存储定时任务数据
type FileCronRepository struct {
	path string
}

func NewFileCronRepository(workspace string) *FileCronRepository {
	return &FileCronRepository{
		path: filepath.Join(workspace, "cron", "jobs.json"),
	}
}

func (r *FileCronRepository) ListJobs(ctx context.Context) ([]model.CronJob, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.CronJob{}, nil
		}
		return nil, fmt.Errorf("cron repo read: %w", err)
	}
	if len(data) == 0 {
		return []model.CronJob{}, nil
	}

	var jobs []model.CronJob
	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, fmt.Errorf("cron repo unmarshal: %w", err)
	}

	for _, job := range jobs {
		if err := job.Validate(); err != nil {
			return nil, fmt.Errorf("cron repo validate job %s: %w", job.ID, err)
		}
	}
	return jobs, nil
}

func (r *FileCronRepository) SaveJobs(jobs []model.CronJob) error {
	for _, job := range jobs {
		if err := job.Validate(); err != nil {
			return fmt.Errorf("cron repo validate job %s: %w", job.ID, err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return fmt.Errorf("cron repo mkdir: %w", err)
	}
	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return fmt.Errorf("cron repo marshal: %w", err)
	}
	if err := os.WriteFile(r.path, data, 0o644); err != nil {
		return fmt.Errorf("cron repo write: %w", err)
	}
	return nil
}
