package runtime

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeCronTriggerService struct {
	triggered int
	err       error
}

func (f *fakeCronTriggerService) TriggerOnce(ctx context.Context) (string, error) {
	f.triggered++
	return "", f.err
}

func TestNewCronRunner(t *testing.T) {
	t.Parallel()

	if _, err := NewCronRunner(nil, 1); err == nil {
		t.Fatal("expected error when service is nil")
	}
	if _, err := NewCronRunner(&fakeCronTriggerService{}, 0); err == nil {
		t.Fatal("expected error when interval is not positive")
	}
}

func TestCronRunner_Run_TriggersService(t *testing.T) {
	t.Parallel()

	service := &fakeCronTriggerService{}
	runner, err := NewCronRunner(service, 1)
	if err != nil {
		t.Fatalf("NewCronRunner() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runner.Run(ctx)
	}()

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if service.triggered > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if service.triggered == 0 {
		t.Fatal("expected cron runner to trigger the service at least once")
	}
}

func TestCronRunner_Run_SwallowsTriggerErrors(t *testing.T) {
	t.Parallel()

	service := &fakeCronTriggerService{err: errors.New("boom")}
	runner, err := NewCronRunner(service, 1)
	if err != nil {
		t.Fatalf("NewCronRunner() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runner.Run(ctx)
	}()

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if service.triggered > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if service.triggered == 0 {
		t.Fatal("expected cron runner to attempt at least one trigger")
	}
}
