package cron

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinybot/internal/domain/model"
)

type cronContextKey string

const cronRepositoryContextKey cronContextKey = "cron-repository"

type fakeCronRepo struct {
	jobs        []model.CronJob
	saved       []model.CronJob
	listErr     error
	saveErr     error
	lastListCtx context.Context
	lastSaveCtx context.Context
}

func (r *fakeCronRepo) ListJobs(ctx context.Context) ([]model.CronJob, error) {
	r.lastListCtx = ctx
	if r.listErr != nil {
		return nil, r.listErr
	}

	out := make([]model.CronJob, len(r.jobs))
	copy(out, r.jobs)
	return out, nil
}

func (r *fakeCronRepo) SaveJobs(ctx context.Context, jobs []model.CronJob) error {
	r.lastSaveCtx = ctx
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = make([]model.CronJob, len(jobs))
	copy(r.saved, jobs)
	return nil
}

type fakeCronAgent struct {
	resp    string
	err     error
	lastCtx context.Context
	calls   []struct{ sessionKey, content string }
}

type fakeResultDispatcher struct {
	err     error
	lastCtx context.Context
	calls   []model.OutboundMessage
}

func (d *fakeResultDispatcher) Dispatch(ctx context.Context, msg model.OutboundMessage) error {
	d.lastCtx = ctx
	d.calls = append(d.calls, msg)
	return d.err
}

func (a *fakeCronAgent) ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error) {
	a.lastCtx = ctx
	a.calls = append(a.calls, struct{ sessionKey, content string }{sessionKey, content})
	return a.resp, a.err
}

func TestNewService(t *testing.T) {
	repo := &fakeCronRepo{}
	agent := &fakeCronAgent{}

	if _, err := NewService(nil, agent, nil); err == nil {
		t.Fatal("expected error when repo is nil")
	}
	if _, err := NewService(repo, nil, nil); err == nil {
		t.Fatal("expected error when agent is nil")
	}
	if _, err := NewService(repo, agent, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	agent := &fakeCronAgent{resp: "done"}

	svc, err := NewService(repo, agent, nil)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}
	if _, err := svc.TriggerOnce(context.Background()); err != nil {
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

func TestService_TriggerOnce_DispatchesResultWhenJobHasDeliveryTarget(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	repo := &fakeCronRepo{
		jobs: []model.CronJob{
			{
				ID:             "job-delivery",
				Name:           "notify",
				Enabled:        true,
				Prompt:         "send summary",
				SessionKey:     "cron:job-delivery",
				DeliverChannel: model.ChannelTelegram,
				DeliverTo:      "chat-42",
				Schedule: model.CronSchedule{
					Kind:         model.CronScheduleEvery,
					EverySeconds: 300,
				},
				NextRunAt: &past,
			},
		},
	}
	agent := &fakeCronAgent{resp: "daily summary ready"}
	dispatcher := &fakeResultDispatcher{}

	svc, err := NewService(repo, agent, dispatcher)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	if _, err := svc.TriggerOnce(context.Background()); err != nil {
		t.Fatalf("TriggerOnce() error: %v", err)
	}

	if len(dispatcher.calls) != 1 {
		t.Fatalf("len(dispatcher.calls) = %d, want 1", len(dispatcher.calls))
	}

	got := dispatcher.calls[0]
	if got.Channel != model.ChannelTelegram {
		t.Fatalf("got.Channel = %q, want %q", got.Channel, model.ChannelTelegram)
	}
	if got.ChatID != "chat-42" {
		t.Fatalf("got.ChatID = %q, want %q", got.ChatID, "chat-42")
	}
	if got.Content != "daily summary ready" {
		t.Fatalf("got.Content = %q, want %q", got.Content, "daily summary ready")
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
				Schedule:   model.CronSchedule{Kind: model.CronScheduleEvery, EverySeconds: 60},
				NextRunAt:  &past,
			},
		},
	}
	agent := &fakeCronAgent{
		resp: "partial output",
		err:  errors.New("agent failed"),
	}

	svc, err := NewService(repo, agent, nil)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}
	if _, err := svc.TriggerOnce(context.Background()); err != nil {
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

func TestService_TriggerOnce_PropagatesContextToRepoAndAgent(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	repo := &fakeCronRepo{
		jobs: []model.CronJob{{
			ID:         "job-ctx",
			Name:       "ctx-job",
			Enabled:    true,
			Prompt:     "check context",
			SessionKey: "cron:job-ctx",
			Schedule:   model.CronSchedule{Kind: model.CronScheduleEvery, EverySeconds: 60},
			NextRunAt:  &past,
		}},
	}
	agent := &fakeCronAgent{resp: "done"}

	svc, err := NewService(repo, agent, nil)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	ctx := context.WithValue(context.Background(), cronRepositoryContextKey, "ctx-value")
	if _, err := svc.TriggerOnce(ctx); err != nil {
		t.Fatalf("TriggerOnce() error: %v", err)
	}

	if repo.lastListCtx == nil {
		t.Fatal("lastListCtx = nil, want non-nil")
	}
	if got := repo.lastListCtx.Value(cronRepositoryContextKey); got != "ctx-value" {
		t.Fatalf("lastListCtx value = %v, want %q", got, "ctx-value")
	}
	if repo.lastSaveCtx == nil {
		t.Fatal("lastSaveCtx = nil, want non-nil")
	}
	if got := repo.lastSaveCtx.Value(cronRepositoryContextKey); got != "ctx-value" {
		t.Fatalf("lastSaveCtx value = %v, want %q", got, "ctx-value")
	}
	if agent.lastCtx == nil {
		t.Fatal("agent.lastCtx = nil, want non-nil")
	}
	if got := agent.lastCtx.Value(cronRepositoryContextKey); got != "ctx-value" {
		t.Fatalf("agent.lastCtx value = %v, want %q", got, "ctx-value")
	}
}

func TestService_TriggerOnce_DisablesAtJobAfterRun(t *testing.T) {
	now := time.Now()
	at := now.Add(-1 * time.Minute)

	repo := &fakeCronRepo{
		jobs: []model.CronJob{
			{
				ID:         "job-at",
				Name:       "one-shot",
				Enabled:    true,
				Prompt:     "send reminder",
				SessionKey: "cron:job-at",
				Schedule: model.CronSchedule{
					Kind: model.CronScheduleAt,
					At:   &at,
				},
				NextRunAt: &at,
			},
		},
	}
	agent := &fakeCronAgent{resp: "done"}

	svc, err := NewService(repo, agent, nil)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	if _, err := svc.TriggerOnce(context.Background()); err != nil {
		t.Fatalf("TriggerOnce() error: %v", err)
	}

	if len(repo.saved) != 1 {
		t.Fatalf("len(repo.saved) = %d, want 1", len(repo.saved))
	}

	got := repo.saved[0]

	if got.Enabled {
		t.Fatal("Enabled = true, want false")
	}
	if got.NextRunAt != nil {
		t.Fatalf("NextRunAt = %v, want nil", got.NextRunAt)
	}
}

func TestService_TriggerOnce_RecordsDispatchError(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)

	repo := &fakeCronRepo{
		jobs: []model.CronJob{
			{
				ID:             "job-dispatch-error",
				Name:           "notify",
				Enabled:        true,
				Prompt:         "send summary",
				SessionKey:     "cron:job-dispatch-error",
				DeliverChannel: model.ChannelTelegram,
				DeliverTo:      "chat-42",
				Schedule: model.CronSchedule{
					Kind:         model.CronScheduleEvery,
					EverySeconds: 300,
				},
				NextRunAt: &past,
			},
		},
	}
	agent := &fakeCronAgent{resp: "daily summary ready"}
	dispatcher := &fakeResultDispatcher{
		err: errors.New("dispatch failed"),
	}

	svc, err := NewService(repo, agent, dispatcher)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	if _, err := svc.TriggerOnce(context.Background()); err != nil {
		t.Fatalf("TriggerOnce() error = %v, want nil", err)
	}

	if len(repo.saved) != 1 {
		t.Fatalf("len(repo.saved) = %d, want 1", len(repo.saved))
	}

	got := repo.saved[0]

	if got.LastResult != "daily summary ready" {
		t.Fatalf("LastResult = %q, want %q", got.LastResult, "daily summary ready")
	}
	if got.LastError == "" {
		t.Fatal("LastError = empty, want non-empty")
	}
}

func TestService_TriggerOnce_SkipsDispatchWithoutDeliveryTarget(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)

	repo := &fakeCronRepo{
		jobs: []model.CronJob{
			{
				ID:         "job-no-delivery",
				Name:       "notify",
				Enabled:    true,
				Prompt:     "send summary",
				SessionKey: "cron:job-no-delivery",
				Schedule: model.CronSchedule{
					Kind:         model.CronScheduleEvery,
					EverySeconds: 300,
				},
				NextRunAt: &past,
			},
		},
	}
	agent := &fakeCronAgent{resp: "daily summary ready"}
	dispatcher := &fakeResultDispatcher{}

	svc, err := NewService(repo, agent, dispatcher)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	if _, err := svc.TriggerOnce(context.Background()); err != nil {
		t.Fatalf("TriggerOnce() error: %v", err)
	}

	if len(dispatcher.calls) != 0 {
		t.Fatalf("len(dispatcher.calls) = %d, want 0", len(dispatcher.calls))
	}
}
