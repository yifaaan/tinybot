package cron

import (
	"context"

	"tinybot/internal/domain/model"
)

// Repository 存取 cron job 列表
type Repository interface {
	ListJobs(ctx context.Context) ([]model.CronJob, error)
	SaveJobs(ctx context.Context, jobs []model.CronJob) error
}

// AgentTurner 负责替 cron job 执行一次 direct chat
type AgentTurner interface {
	ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error)
}

// ResultDispatcher 负责将 cron job 的结果分发到对应的Channel
type ResultDispatcher interface {
	Dispatch(ctx context.Context, msg model.OutboundMessage) error
}
