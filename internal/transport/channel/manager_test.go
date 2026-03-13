package channel

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"tinybot/internal/domain/model"
	"tinybot/internal/transport"
	transportbus "tinybot/internal/transport/bus"
)

type fakeChannel struct {
	name      model.Channel
	startErr  error
	sendErr   error
	started   chan struct{}
	sentMu    sync.Mutex
	sent      []model.OutboundMessage
	waitOnCtx bool
}

func newFakeChannel(name model.Channel) *fakeChannel {
	return &fakeChannel{
		name:    name,
		started: make(chan struct{}, 1),
	}
}

func (f *fakeChannel) Name() model.Channel {
	return f.name
}

func (f *fakeChannel) Start(ctx context.Context) error {
	select {
	case f.started <- struct{}{}:
	default:
	}
	if f.startErr != nil {
		return f.startErr
	}
	if f.waitOnCtx {
		<-ctx.Done()
	}
	return nil
}

func (f *fakeChannel) Send(ctx context.Context, out model.OutboundMessage) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	f.sentMu.Lock()
	f.sent = append(f.sent, out)
	f.sentMu.Unlock()
	return nil
}

func (f *fakeChannel) Sent() []model.OutboundMessage {
	f.sentMu.Lock()
	defer f.sentMu.Unlock()
	cp := make([]model.OutboundMessage, len(f.sent))
	copy(cp, f.sent)
	return cp
}

func TestChannelManager_Start_NoChannels(t *testing.T) {
	t.Parallel()

	manager := NewChannelManager(transportbus.NewMemoryBus(1))
	err := manager.Start(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestChannelManager_DispatchesOutboundToMatchingChannel(t *testing.T) {
	t.Parallel()

	b := transportbus.NewMemoryBus(1)
	manager := NewChannelManager(b)
	ch := newFakeChannel(model.ChannelCLI)
	ch.waitOnCtx = true
	manager.RegisterChannel(ch)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer b.Close()

	done := make(chan error, 1)
	go func() {
		done <- manager.Start(ctx)
	}()

	select {
	case <-ch.started:
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not start")
	}

	want := model.OutboundMessage{
		Channel: model.ChannelCLI,
		ChatID:  "gateway",
		Content: "hello",
	}
	if err := b.PublishOutbound(ctx, want); err != nil {
		t.Fatalf("PublishOutbound() error: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(ch.Sent()) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	got := ch.Sent()
	if len(got) != 1 {
		t.Fatalf("sent len = %d, want 1", len(got))
	}
	if got[0].Content != want.Content {
		t.Fatalf("sent content = %q, want %q", got[0].Content, want.Content)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Start() error: %v", err)
	}
}

func TestChannelManager_Start_PropagatesChannelError(t *testing.T) {
	t.Parallel()

	b := transportbus.NewMemoryBus(1)
	manager := NewChannelManager(b)
	ch := newFakeChannel(model.ChannelCLI)
	ch.startErr = errors.New("start failed")
	manager.RegisterChannel(ch)

	err := manager.Start(context.Background())
	if err == nil || !errors.Is(err, ch.startErr) {
		t.Fatalf("expected wrapped start error, got %v", err)
	}
}

func TestChannelManager_Start_IgnoresUnknownOutboundChannel(t *testing.T) {
	t.Parallel()

	b := transportbus.NewMemoryBus(1)
	manager := NewChannelManager(b)
	ch := newFakeChannel(model.ChannelCLI)
	ch.waitOnCtx = true
	manager.RegisterChannel(ch)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer b.Close()

	done := make(chan error, 1)
	go func() {
		done <- manager.Start(ctx)
	}()

	select {
	case <-ch.started:
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not start")
	}

	if err := b.PublishOutbound(ctx, model.OutboundMessage{
		Channel: model.ChannelTelegram,
		ChatID:  "other",
		Content: "ignored",
	}); err != nil {
		t.Fatalf("PublishOutbound() error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if len(ch.Sent()) != 0 {
		t.Fatalf("expected no messages sent, got %d", len(ch.Sent()))
	}

	cancel()
	if err := <-done; err != nil && !errors.Is(err, transport.ErrBusClosed) {
		t.Fatalf("Start() error: %v", err)
	}
}
