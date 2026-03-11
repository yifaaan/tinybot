package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"tinybot/internal/adapters/bus"
	"tinybot/internal/domain/model"
)

type fakeProcessor struct {
	out  model.OutboundMessage
	err  error
	seen []model.InboundMessage
	mu   sync.Mutex
}

func (p *fakeProcessor) ProcessMessage(_ context.Context, msg model.InboundMessage) (model.OutboundMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.seen = append(p.seen, msg)
	return p.out, p.err
}

func (p *fakeProcessor) Seen() []model.InboundMessage {
	p.mu.Lock()
	defer p.mu.Unlock()

	cp := make([]model.InboundMessage, len(p.seen))
	copy(cp, p.seen)
	return cp
}

func TestLoop_FrowardsInboundToOutbound(t *testing.T) {
	t.Parallel()

	bus := bus.NewMemoryBus(16)
	processor := &fakeProcessor{
		out: model.OutboundMessage{
			Channel: "test_channel",
			ChatID:  "test_chat_id",
			Content: "test response",
		},
	}
	loop := NewLoop(processor, bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer bus.Close()

	done := make(chan error, 1)
	go func() {
		done <- loop.Run(ctx)
	}()

	inbound := model.InboundMessage{
		ID:      "msg-1",
		Channel: model.ChannelCLI,
		ChatID:  "direct",
		Content: "hello",
	}

	if err := bus.PublishInbound(ctx, inbound); err != nil {

		t.Fatalf("publish inbound: %v", err)
	}
	outboundCtx, outboundCancel := context.WithTimeout(ctx, 2*time.Second)
	defer outboundCancel()

	got, err := bus.ConsumeOutbound(outboundCtx)
	if err != nil {
		t.Fatalf("ConsumeOutbound() error: %v", err)
	}

	if got.Content != processor.out.Content {
		t.Fatalf("expected content %q, got %q", processor.out.Content, got.Content)
	}

	_ = done
}

func TestLoop_PublishesFallbackWhenProcessorFails(t *testing.T) {
	t.Parallel()

	b := bus.NewMemoryBus(1)
	processor := &fakeProcessor{

		err: errors.New("boom"),
	}

	loop := NewLoop(processor, b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer b.Close()

	go func() {
		_ = loop.Run(ctx)
	}()

	inbound := model.InboundMessage{
		ID:      "msg-2",
		Channel: model.ChannelCLI,
		ChatID:  "direct",
		Content: "trigger error",
	}

	err := b.PublishInbound(ctx, inbound)
	if err != nil {
		t.Fatalf("PublishInbound() error: %v", err)
	}
	msg, err := b.ConsumeInbound(ctx)
	if err != nil {
		t.Fatalf("ConsumeInbound() error: %v", err)
	}
	_, err = processor.ProcessMessage(ctx, msg)

	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error containing 'boom', got: %v", err)
	}
}
