package agent

import (
	"context"
	"testing"
	"time"
	"tinybot/internal/adapters/bus"
	"tinybot/internal/domain/model"
)

type fackProcessor struct {
	out  model.OutboundMessage
	err  error
	seen []model.InboundMessage
}

func (p *fackProcessor) ProcessMessage(_ context.Context, msg model.InboundMessage) (model.OutboundMessage, error) {
	p.seen = append(p.seen, msg)
	return p.out, p.err
}

func TestLoop_FrowardsInboundToOutbound(t *testing.T) {
	t.Parallel()

	bus := bus.NewMemoryBus(16)
	processor := &fackProcessor{
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
