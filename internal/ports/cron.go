package ports

import (
	"context"
	"tinybot/internal/domain/model"
)

type CronRepository interface {
	ListJobs(ctx context.Context) ([]model.CronJob, error)
	SaveJobs(jobs []model.CronJob) error
}
