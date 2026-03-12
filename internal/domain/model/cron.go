package model

import (
	"fmt"
	"time"
)

type CronScheduleKind string

const (
	CronScheduleEvery CronScheduleKind = "every"
)

// CronSchedule represents the schedule of a cron job.
type CronSchedule struct {
	Kind         CronScheduleKind `json:"kind"`
	EverySeconds int              `json:"every_seconds,omitempty"`
}

type CronJob struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Enabled    bool         `json:"enabled"`
	Schedule   CronSchedule `json:"schedule"`
	Prompt     string       `json:"prompt"`
	DeliverTo  string       `json:"deliver_to,omitempty"`
	SessionKey string       `json:"session_key"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	NextRunAt *time.Time `json:"next_run_at"`
	LastRunAt *time.Time `json:"last_run_at"`

	LastError  string `json:"last_error,omitempty"`
	LastResult string `json:"last_result,omitempty"`
}

// IsDue reports whether the job should run at the given time.
func (j CronJob) IsDue(now time.Time) bool {
	return j.Enabled && j.NextRunAt != nil && !now.Before(*j.NextRunAt)
}

func (j *CronJob) Validate() error {
	if j.Name == "" {
		return fmt.Errorf("cron job name is required")
	}
	if j.ID == "" {
		return fmt.Errorf("cron job ID is required")
	}
	if j.Prompt == "" {
		return fmt.Errorf("cron job prompt is required")
	}
	if j.Schedule.Kind != CronScheduleEvery {
		return fmt.Errorf("unsupported cron schedule kind: %s", j.Schedule.Kind)
	}
	if j.Schedule.EverySeconds <= 0 {
		return fmt.Errorf("cron job schedule every_seconds must be positive")
	}
	return nil
}

// ComputeNextRun computes the next run time based on the job's schedule and the given time.
func (j *CronJob) ComputeNextRun(from time.Time) *time.Time {
	switch j.Schedule.Kind {
	case CronScheduleEvery:
		nextRun := from.Add(time.Duration(j.Schedule.EverySeconds) * time.Second)
		return &nextRun
	default:
		return nil
	}
}
