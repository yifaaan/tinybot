package cron

import (
	"context"
	"errors"
	"time"
	"tinybot/internal/domain/model"
)

// Service evaluates due cron jobs and updates their execution state.
//
// Responsibilities:
//   - load the current cron jobs from the repository
//   - execute due jobs through the chat service
//   - record timestamps, results, errors, and the next run time
//
// Inputs: the current context plus repository state.
// Outputs: a status message placeholder and an error when the trigger pass fails.
// State changes: due jobs get updated LastRunAt, LastResult, LastError, UpdatedAt, and NextRunAt values.
// Side effects: repository writes and delegated chat-service calls.
// Compatibility: preserves the existing Go cron-trigger behavior while separating scheduling from periodic runtime concerns.
type Service struct {
	repo  Repository
	agent AgentTurner
}

// NewService constructs the cron service from explicit dependencies.
func NewService(repo Repository, agent AgentTurner) (*Service, error) {
	if repo == nil {
		return nil, errors.New("cron service: repo is required")
	}
	if agent == nil {
		return nil, errors.New("cron service: agent is required")
	}
	return &Service{
		repo:  repo,
		agent: agent,
	}, nil
}

// TriggerOnce evaluates the cron store once and executes every due job.
func (s *Service) TriggerOnce(ctx context.Context) (string, error) {
	jobs, err := s.repo.ListJobs(ctx)
	if err != nil {
		return "", err
	}

	changed := false
	now := time.Now()

	for i := range jobs {
		job := &jobs[i]
		if !job.IsDue(now) {
			continue
		}

		resp, err := s.agent.ProcessDirect(ctx, job.SessionKey, job.Prompt)
		runAt := time.Now()
		job.LastRunAt = &runAt
		job.UpdatedAt = runAt

		if err != nil {
			job.LastError = err.Error()
			job.LastResult = resp
		} else {
			job.LastError = ""
			job.LastResult = resp
		}

		s.advanceJobAfterRun(job, runAt)

		changed = true
	}

	if !changed {
		return "", nil
	}
	if err := s.repo.SaveJobs(ctx, jobs); err != nil {
		return "", err
	}
	return "", nil
}

// advanceJobAfterRun 负责执行后的状态迁移
func (s *Service) advanceJobAfterRun(job *model.CronJob, runAt time.Time) {
	if job == nil {
		return
	}

	switch job.Schedule.Kind {
	case model.CronScheduleEvery:
		job.NextRunAt = job.ComputeNextRun(runAt)
	case model.CronScheduleAt:
		job.Enabled = false
		job.NextRunAt = nil
	default:
		job.NextRunAt = nil
	}
}
