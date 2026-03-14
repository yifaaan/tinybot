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

func TestCronJob_Validate_WithAtSchedule(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		job     CronJob
		wantErr bool
	}{
		{
			name: "valid at schedule",
			job: CronJob{
				ID:      "job-at",
				Name:    "one-shot",
				Prompt:  "remind me",
				Enabled: true,
				Schedule: CronSchedule{
					Kind: CronScheduleAt,
					At:   &now,
				},
			},
			wantErr: false,
		},
		{
			name: "missing at time",
			job: CronJob{
				ID:      "job-at",
				Name:    "one-shot",
				Prompt:  "remind me",
				Enabled: true,
				Schedule: CronSchedule{
					Kind: CronScheduleAt,
					At:   nil,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCronJob_Validate_WithDeliveryTarget(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		job     CronJob
		wantErr bool
	}{
		{
			name: "no delivery target is valid",
			job: CronJob{
				ID:      "job-no-delivery",
				Name:    "daily-check",
				Prompt:  "check inbox",
				Enabled: true,
				Schedule: CronSchedule{
					Kind: CronScheduleAt,
					At:   &now,
				},
			},
			wantErr: false,
		},
		{
			name: "complete delivery target is valid",
			job: CronJob{
				ID:             "job-delivery",
				Name:           "notify-telegram",
				Prompt:         "send summary",
				Enabled:        true,
				DeliverChannel: ChannelTelegram,
				DeliverTo:      "chat-42",
				Schedule: CronSchedule{
					Kind: CronScheduleAt,
					At:   &now,
				},
			},
			wantErr: false,
		},
		{
			name: "recipient without channel is invalid",
			job: CronJob{
				ID:        "job-missing-channel",
				Name:      "bad-delivery",
				Prompt:    "send summary",
				Enabled:   true,
				DeliverTo: "chat-42",
				Schedule: CronSchedule{
					Kind: CronScheduleAt,
					At:   &now,
				},
			},
			wantErr: true,
		},
		{
			name: "channel without recipient is invalid",
			job: CronJob{
				ID:             "job-missing-recipient",
				Name:           "bad-delivery",
				Prompt:         "send summary",
				Enabled:        true,
				DeliverChannel: ChannelTelegram,
				Schedule: CronSchedule{
					Kind: CronScheduleAt,
					At:   &now,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCronJob_ComputeNextRun_WithAtSchedule(t *testing.T) {
	base := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)
	future := base.Add(30 * time.Minute)
	past := base.Add(-30 * time.Minute)

	tests := []struct {
		name string
		job  CronJob
		from time.Time
		want *time.Time
	}{
		{
			name: "future at returns same absolute time",
			job: CronJob{
				Schedule: CronSchedule{
					Kind: CronScheduleAt,
					At:   &future,
				},
			},
			from: base,
			want: &future,
		},
		{
			name: "past at returns nil",
			job: CronJob{
				Schedule: CronSchedule{
					Kind: CronScheduleAt,
					At:   &past,
				},
			},
			from: base,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.job.ComputeNextRun(tt.from)
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("ComputeNextRun() got = %v, want %v", got, tt.want)
			}
		})
	}
}
