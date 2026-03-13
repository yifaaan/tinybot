package cronrepo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
	"tinybot/internal/domain/model"
)

func newTestWorkspace(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func writeCronFile(t *testing.T, workspace string, content string) string {
	t.Helper()

	path := filepath.Join(workspace, "cron", "jobs.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create cron directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", path, err)
	}
	return path
}

func TestFileCronRepository_ListJobs_FileNotExist(t *testing.T) {
	workspace := newTestWorkspace(t)
	repo := NewFileCronRepository(workspace)

	got, err := repo.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no jobs, got %d", len(got))
	}
}

func TestFileCronRepository_ListJobs_EmptyFile(t *testing.T) {
	workspace := newTestWorkspace(t)
	repo := NewFileCronRepository(workspace)
	writeCronFile(t, workspace, "")

	got, err := repo.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no jobs, got %d", len(got))
	}
}

func TestFileCronRepository_ListJobs_InvalidJSON(t *testing.T) {
	workspace := newTestWorkspace(t)
	repo := NewFileCronRepository(workspace)
	writeCronFile(t, workspace, "{invalid json}")

	_, err := repo.ListJobs(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFileCronRepository_ListJobs_CanceledContext(t *testing.T) {
	workspace := newTestWorkspace(t)
	repo := NewFileCronRepository(workspace)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.ListJobs(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFileCronRepository_SaveJobs_ThenListJobs(t *testing.T) {
	workspace := newTestWorkspace(t)
	repo := NewFileCronRepository(workspace)
	now := time.Now()
	next := now.Add(5 * time.Minute)
	want := []model.CronJob{
		{
			ID:         "job-1",
			Name:       "daily-check",
			Enabled:    true,
			Prompt:     "check project status",
			SessionKey: "cron:job-1",
			Schedule: model.CronSchedule{
				Kind:         model.CronScheduleEvery,
				EverySeconds: 300,
			},
			CreatedAt: now,
			UpdatedAt: now,
			NextRunAt: &next,
		},
	}
	if err := repo.SaveJobs(context.Background(), want); err != nil {
		t.Fatalf("SaveJobs() error: %v", err)
	}
	got, err := repo.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].ID != want[0].ID {
		t.Fatalf("got[0].ID = %q, want %q", got[0].ID, want[0].ID)
	}
	if got[0].Name != want[0].Name {
		t.Fatalf("got[0].Name = %q, want %q", got[0].Name, want[0].Name)
	}
	if got[0].Prompt != want[0].Prompt {
		t.Fatalf("got[0].Prompt = %q, want %q", got[0].Prompt, want[0].Prompt)
	}
	if got[0].Schedule.EverySeconds != want[0].Schedule.EverySeconds {
		t.Fatalf("got[0].Schedule.EverySeconds = %d, want %d", got[0].Schedule.EverySeconds, want[0].Schedule.EverySeconds)
	}
}

func TestFileCronRepository_SaveJobs_CanceledContext(t *testing.T) {
	workspace := newTestWorkspace(t)
	repo := NewFileCronRepository(workspace)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := repo.SaveJobs(ctx, []model.CronJob{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFileCronRepository_SaveJobs_ThenListJobs_WithAtSchedule(t *testing.T) {
	workspace := newTestWorkspace(t)
	repo := NewFileCronRepository(workspace)

	now := time.Now().UTC().Truncate(time.Second)
	at := now.Add(2 * time.Hour)

	want := []model.CronJob{
		{
			ID:         "job-at",
			Name:       "one-shot",
			Enabled:    true,
			Prompt:     "future reminder",
			SessionKey: "cron:job-at",
			Schedule: model.CronSchedule{
				Kind: model.CronScheduleAt,
				At:   &at,
			},
			CreatedAt: now,
			UpdatedAt: now,
			NextRunAt: &at,
		},
	}

	if err := repo.SaveJobs(context.Background(), want); err != nil {
		t.Fatalf("SaveJobs() error: %v", err)
	}

	got, err := repo.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	if got[0].Schedule.Kind != want[0].Schedule.Kind {
		t.Fatalf("got[0].Schedule.Kind = %q, want %q", got[0].Schedule.Kind, want[0].Schedule.Kind)
	}
	if got[0].Schedule.At == nil {
		t.Fatal("got[0].Schedule.At = nil, want non-nil")
	}
	if !got[0].Schedule.At.Equal(*want[0].Schedule.At) {
		t.Fatalf("got[0].Schedule.At = %v, want %v", got[0].Schedule.At, want[0].Schedule.At)
	}
}
