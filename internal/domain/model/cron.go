package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

type CronScheduleKind string

const (
	CronScheduleEvery CronScheduleKind = "every"
	CronScheduleAt    CronScheduleKind = "at"
	CronScheduleCron  CronScheduleKind = "cron"
)

// CronSchedule describes when a job should run.
type CronSchedule struct {
	Kind         CronScheduleKind `json:"kind"`
	EverySeconds int              `json:"every_seconds,omitempty"`
	At           *time.Time       `json:"at,omitempty"`
	CronExpr     string           `json:"cron_expr,omitempty"`
}

type CronJob struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Enabled        bool         `json:"enabled"`
	Schedule       CronSchedule `json:"schedule"`
	Prompt         string       `json:"prompt"`
	DeliverChannel Channel      `json:"deliver_channel,omitempty"`
	DeliverTo      string       `json:"deliver_to,omitempty"`
	SessionKey     string       `json:"session_key"`

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

// HasDeliveryTarget reports whether the job is configured to deliver its
// result to a concrete channel target.
func (j CronJob) HasDeliveryTarget() bool {
	return strings.TrimSpace(string(j.DeliverChannel)) != "" && strings.TrimSpace(j.DeliverTo) != ""
}

func (j CronJob) validateDeliveryTarget() error {
	channel := strings.TrimSpace(string(j.DeliverChannel))
	to := strings.TrimSpace(j.DeliverTo)

	switch {
	case channel == "" && to == "":
		return nil
	case channel == "":
		return fmt.Errorf("cron job deliver_channel is required when deliver_to is set")
	case to == "":
		return fmt.Errorf("cron job deliver_to is required when deliver_channel is set")
	default:
		return nil
	}
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
	if err := j.validateDeliveryTarget(); err != nil {
		return err
	}

	switch j.Schedule.Kind {
	case CronScheduleEvery:
		if j.Schedule.EverySeconds <= 0 {
			return fmt.Errorf("cron job schedule every_seconds must be positive")
		}
	case CronScheduleAt:
		if j.Schedule.At == nil {
			return fmt.Errorf("cron job schedule at is required")
		}
		if j.Schedule.At.IsZero() {
			return fmt.Errorf("cron job schedule at must be a valid time")
		}
	case CronScheduleCron:
		if j.Schedule.CronExpr == "" {
			return fmt.Errorf("cron job schedule cron_expr is required")
		}
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		_, err := parser.Parse(j.Schedule.CronExpr)
		if err != nil {
			return fmt.Errorf("cron job schedule: %w", err)
		}
	default:
		return fmt.Errorf("unsupported cron schedule kind: %s", j.Schedule.Kind)
	}

	return nil
}

// ComputeNextRun computes the next execution time for the configured schedule.
func (j *CronJob) ComputeNextRun(from time.Time) *time.Time {
	switch j.Schedule.Kind {
	case CronScheduleEvery:
		nextRun := from.Add(time.Duration(j.Schedule.EverySeconds) * time.Second)
		return &nextRun
	case CronScheduleAt:
		if j.Schedule.At == nil || j.Schedule.At.Before(from) {
			return nil
		}
		nextRun := *j.Schedule.At
		return &nextRun
	case CronScheduleCron:
		sche, err := cron.ParseStandard(j.Schedule.CronExpr)
		if err != nil {
			return nil
		}
		nextRun := sche.Next(from)
		return &nextRun
	default:
		return nil
	}
}
