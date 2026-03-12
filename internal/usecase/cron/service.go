package cron

import (
	"context"
	"errors"
	"time"
	"tinybot/internal/ports"
)

type AgentTurner interface {
	ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error)
}

type Service struct {
	repo     ports.CronRepository
	agent    AgentTurner
	inverval time.Duration
}

func NewService(repo ports.CronRepository, agent AgentTurner, intervalSeconds int) (*Service, error) {
	if repo == nil {
		return nil, errors.New("cron service: repo is required")
	}

	if agent == nil {
		return nil, errors.New("cron service: agent is required")
	}

	if intervalSeconds <= 0 {
		return nil, errors.New("cron service: intervalSeconds must be positive")
	}
	return &Service{
		repo:     repo,
		agent:    agent,
		inverval: time.Duration(intervalSeconds) * time.Second,
	}, nil
}

func (s *Service) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.inverval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := s.TriggerOnce(ctx); err != nil {
				// TODO: log error
				continue
			}
		}
	}
}

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

		job.NextRunAt = job.ComputeNextRun(runAt)
		changed = true
	}

	if !changed {
		return "", nil
	}
	if err := s.repo.SaveJobs(jobs); err != nil {
		return "", err
	}
	return "", nil
}
