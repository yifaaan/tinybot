package cron

import (
	"context"
	"errors"
	"testing"
	"time"
	"tinybot/internal/domain/model"
)

type fakeCronRepo struct {
	jobs    []model.CronJob
	saved   []model.CronJob
	listErr error
	saveErr error
}

func (r *fakeCronRepo) ListJobs(_ context.Context) ([]model.CronJob, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}

	out := make([]model.CronJob, len(r.jobs))
	copy(out, r.jobs)
	return out, nil
}

func (r *fakeCronRepo) SaveJobs(jobs []model.CronJob) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = make([]model.CronJob, len(jobs))
	copy(r.saved, jobs)
	return nil
}

type fakeAgent struct {
	resp  string
	err   error
	calls []struct{ sessionKey, content string }
}

func (a *fakeAgent) ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error) {
	a.calls = append(a.calls, struct{ sessionKey, content string }{sessionKey, content})
	return a.resp, a.err
}

func TestService_TriggerOnce_ExecutesDueJob(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Minute)

	repo := &fakeCronRepo{
		jobs: []model.CronJob{
			{
				ID:         "job1",
				Name:       "Test Job",
				Enabled:    true,
				SessionKey: "cron:job-1",
				Prompt:     "check project status",
				Schedule:   model.CronSchedule{Kind: model.CronScheduleEvery, EverySeconds: 300},
				NextRunAt:  &past,
			},
		},
	}

	agent := &fakeAgent{
		resp: "done",
	}

	svc, err := NewService(repo, agent, 30)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}
	_, err = svc.TriggerOnce(context.Background())
	if err != nil {
		t.Fatalf("TriggerOnce() error: %v", err)
	}
	if len(agent.calls) != 1 {
		t.Fatalf("len(agent.calls) = %d, want 1", len(agent.calls))
	}
	if agent.calls[0].sessionKey != "cron:job-1" {
		t.Fatalf("sessionKey = %q, want %q", agent.calls[0].sessionKey, "cron:job-1")
	}
	if agent.calls[0].content != "check project status" {
		t.Fatalf("content = %q, want %q", agent.calls[0].content, "check project status")
	}
	if len(repo.saved) != 1 {
		t.Fatalf("len(repo.saved) = %d, want 1", len(repo.saved))
	}
	got := repo.saved[0]
	if got.LastRunAt == nil {
		t.Fatal("LastRunAt = nil, want non-nil")
	}
	if got.NextRunAt == nil {
		t.Fatal("NextRunAt = nil, want non-nil")
	}
	if got.LastResult != "done" {
		t.Fatalf("LastResult = %q, want %q", got.LastResult, "done")
	}
	if got.LastError != "" {
		t.Fatalf("LastError = %q, want empty", got.LastError)
	}
}

func TestService_TriggerOnce_RecordsAgentError(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	repo := &fakeCronRepo{
		jobs: []model.CronJob{
			{
				ID:         "job-1",
				Name:       "error-job",
				Enabled:    true,
				Prompt:     "please fail",
				SessionKey: "cron:job-1",
				Schedule: model.CronSchedule{
					Kind:         model.CronScheduleEvery,
					EverySeconds: 60,
				},
				NextRunAt: &past,
			},
		},
	}
	agent := &fakeAgent{
		resp: "partial output",
		err:  errors.New("agent failed"),
	}
	svc, err := NewService(repo, agent, 30)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}
	_, err = svc.TriggerOnce(context.Background())
	if err != nil {
		t.Fatalf("TriggerOnce() error: %v", err)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("len(repo.saved) = %d, want 1", len(repo.saved))
	}
	got := repo.saved[0]
	if got.LastRunAt == nil {
		t.Fatal("LastRunAt = nil, want non-nil")
	}
	if got.LastResult != "partial output" {
		t.Fatalf("LastResult = %q, want %q", got.LastResult, "partial output")
	}
	if got.LastError == "" {
		t.Fatal("LastError = empty, want non-empty")
	}
	if got.NextRunAt == nil {
		t.Fatal("NextRunAt = nil, want non-nil")
	}
}
