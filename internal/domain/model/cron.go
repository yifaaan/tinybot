package model

import "time"

type CronJob struct {
	ID        string
	Name      string
	Enabled   bool
	Schedule  string
	Payload   string
	DeliverTo string
	NextRunAt *time.Time
	LastRunAt *time.Time
}

// IsDue reports whether the job should run at the given time.
func (j CronJob) IsDue(now time.Time) bool {
	return j.Enabled && j.NextRunAt != nil && !now.Before(*j.NextRunAt)
}
