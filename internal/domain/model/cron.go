package model

import (
	"fmt"
	"time"
)

type CronScheduleKind string

const (
	CronScheduleEvery CronScheduleKind = "every"
	CronScheduleAt    CronScheduleKind = "at"
	// TODO: cron
)

// CronSchedule 任务按什么方式触发
type CronSchedule struct {
	Kind         CronScheduleKind `json:"kind"`
	EverySeconds int              `json:"every_seconds,omitempty"`

	// At 表示在某个时间点触发一次
	At *time.Time `json:"at,omitempty"`
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
	default:
		return fmt.Errorf("unsupported cron schedule kind: %s", j.Schedule.Kind)
	}

	return nil
}

// ComputeNextRun 根据调度类型计算“下一次执行时间
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
	default:
		return nil
	}
}
