package model

import (
	"testing"
	"time"
)

func TestCronJob_IsDue(t *testing.T) {
	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	tests := []struct {
		name string
		job  CronJob
		want bool
	}{
		{
			name: "enabled and past due",
			job:  CronJob{Enabled: true, NextRunAt: &past},
			want: true,
		},
		{
			name: "enabled and exactly now",
			job:  CronJob{Enabled: true, NextRunAt: &now},
			want: true,
		},
		{
			name: "enabled but future",
			job:  CronJob{Enabled: true, NextRunAt: &future},
			want: false,
		},
		{
			name: "disabled even if past due",
			job:  CronJob{Enabled: false, NextRunAt: &past},
			want: false,
		},
		{
			name: "enabled but nil NextRunAt",
			job:  CronJob{Enabled: true, NextRunAt: nil},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.job.IsDue(now); got != tt.want {
				t.Errorf("IsDue() = %v, want %v", got, tt.want)
			}
		})
	}
}
