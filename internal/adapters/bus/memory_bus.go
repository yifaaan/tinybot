package bus

import (
	"context"
	"fmt"
	"sync"
	"tinybot/internal/domain/model"
	"tinybot/internal/ports"
)

type MemoryBus struct {
	inbound  chan model.InboundMessage
	outbound chan model.OutboundMessage

	closed chan struct{}

	closeOnce sync.Once
}

func NewMemoryBus(bufferSize int) *MemoryBus {
	if bufferSize <= 0 {
		bufferSize = 16
	}
	return &MemoryBus{
		inbound:  make(chan model.InboundMessage, bufferSize),
		outbound: make(chan model.OutboundMessage, bufferSize),
		closed:   make(chan struct{}),
	}
}

func (b *MemoryBus) PublishInbound(ctx context.Context, msg model.InboundMessage) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("publish inbound: %w", ctx.Err())
	case <-b.closed:
		return ports.ErrBusClosed
	case b.inbound <- msg:
		return nil
	}
}

func (b *MemoryBus) ConsumeInbound(ctx context.Context) (model.InboundMessage, error) {
	select {
	case <-ctx.Done():
		return model.InboundMessage{}, fmt.Errorf("consume inbound: %w", ctx.Err())
	case <-b.closed:
		return model.InboundMessage{}, ports.ErrBusClosed
	case msg := <-b.inbound:
		return msg, nil
	}
}

func (b *MemoryBus) PublishOutbound(ctx context.Context, msg model.OutboundMessage) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("publish outbound: %w", ctx.Err())
	case <-b.closed:
		return ports.ErrBusClosed
	case b.outbound <- msg:
		return nil
	}
}

func (b *MemoryBus) ConsumeOutbound(ctx context.Context) (model.OutboundMessage, error) {
	select {
	case <-ctx.Done():
		return model.OutboundMessage{}, fmt.Errorf("consume outbound: %w", ctx.Err())
	case <-b.closed:
		return model.OutboundMessage{}, ports.ErrBusClosed
	case msg := <-b.outbound:
		return msg, nil
	}
}

func (b *MemoryBus) Close() error {
	b.closeOnce.Do(func() {
		close(b.inbound)
		close(b.outbound)
		close(b.closed)
	})
	return nil
}
