package runtime

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeHeartbeatTriggerService struct {
	triggered int
	err       error
}

func (f *fakeHeartbeatTriggerService) TriggerOnce(ctx context.Context) (string, error) {
	f.triggered++
	return "", f.err
}

func TestNewHeartbeatRunner(t *testing.T) {
	t.Parallel()

	if _, err := NewHeartbeatRunner(nil, 1, true); err == nil {
		t.Fatal("expected error when service is nil")
	}
	if _, err := NewHeartbeatRunner(&fakeHeartbeatTriggerService{}, 0, true); err == nil {
		t.Fatal("expected error when interval is not positive")
	}
}

func TestHeartbeatRunner_Run_ReturnsImmediatelyWhenDisabled(t *testing.T) {
	t.Parallel()

	runner, err := NewHeartbeatRunner(&fakeHeartbeatTriggerService{}, 0, false)
	if err != nil {
		t.Fatalf("NewHeartbeatRunner() error: %v", err)
	}
	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestHeartbeatRunner_Run_TriggersService(t *testing.T) {
	t.Parallel()

	service := &fakeHeartbeatTriggerService{}
	runner, err := NewHeartbeatRunner(service, 1, true)
	if err != nil {
		t.Fatalf("NewHeartbeatRunner() error: %v", err)
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
		t.Fatal("expected heartbeat runner to trigger the service at least once")
	}
}

func TestHeartbeatRunner_Run_SwallowsTriggerErrors(t *testing.T) {
	t.Parallel()

	service := &fakeHeartbeatTriggerService{err: errors.New("boom")}
	runner, err := NewHeartbeatRunner(service, 1, true)
	if err != nil {
		t.Fatalf("NewHeartbeatRunner() error: %v", err)
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
		t.Fatal("expected heartbeat runner to attempt at least one trigger")
	}
}
