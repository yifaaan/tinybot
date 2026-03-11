package bus

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinybot/internal/domain/model"
	"tinybot/internal/ports"
)

func TestMemoryBus_InboundRoundTrip(t *testing.T) {
	t.Parallel()

	b := NewMemoryBus(1)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	want := model.InboundMessage{
		ID:      "msg-1",
		Channel: model.ChannelCLI,
		ChatID:  "direct",
		Content: "hello",
	}

	b.PublishInbound(ctx, want)
	got, err := b.ConsumeInbound(ctx)
	if err != nil {
		t.Fatalf("ConsumeInbound() error: %v", err)
	}
	if got.Content != want.Content || got.Channel != want.Channel || got.ChatID != want.ChatID {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMemoryBus_OutboundRoundTrip(t *testing.T) {
	t.Parallel()

	b := NewMemoryBus(1)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	want := model.OutboundMessage{
		Channel: model.ChannelCLI,
		ChatID:  "direct",
		Content: "hello back",
	}

	if err := b.PublishOutbound(ctx, want); err != nil {
		t.Fatalf("PublishOutbound() error: %v", err)
	}
	got, err := b.ConsumeOutbound(ctx)
	if err != nil {
		t.Fatalf("ConsumeOutbound() error: %v", err)
	}
	if got.Content != want.Content || got.Channel != want.Channel || got.ChatID != want.ChatID {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMemoryBus_CloseReturnsErrBusClosed(t *testing.T) {
	t.Parallel()

	b := NewMemoryBus(1)
	if err := b.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// 1. PublishInbound -> should return ErrBusClosed.
	err := b.PublishInbound(ctx, model.InboundMessage{})
	if !errors.Is(err, ports.ErrBusClosed) {
		t.Fatal("expected ErrBusClosed")
	}
	// 2. PublishOutbound -> should return ErrBusClosed.
	err = b.PublishOutbound(ctx, model.OutboundMessage{})
	if !errors.Is(err, ports.ErrBusClosed) {
		t.Fatal("expected ErrBusClosed")
	}
	// 3. ConsumeInbound -> should return ErrBusClosed.
	_, err = b.ConsumeInbound(ctx)
	if !errors.Is(err, ports.ErrBusClosed) {
		t.Fatal("expected ErrBusClosed")
	}
	// 4. ConsumeOutbound -> should return ErrBusClosed.
	_, err = b.ConsumeOutbound(ctx)
	if !errors.Is(err, ports.ErrBusClosed) {
		t.Fatal("expected ErrBusClosed")
	}
}

func TestMemoryBus_ContextCancellation(t *testing.T) {
	t.Parallel()

	b := NewMemoryBus(1)

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err := b.PublishInbound(canceledCtx, model.InboundMessage{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected wrapped context.Canceled, got %v", err)
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer timeoutCancel()

	_, err = b.ConsumeOutbound(timeoutCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected wrapped context.DeadlineExceeded, got %v", err)
	}
}

