package bus

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"tinybot/internal/domain/model"
	"tinybot/internal/transport"
)

// MemoryBus is the in-memory transport bus used by the local gateway runtime.
//
// Responsibilities:
//   - buffer inbound and outbound messages in-process
//   - coordinate graceful shutdown for the gateway loop and channels
//
// Inputs: inbound/outbound domain messages plus contexts for cancellation.
// Outputs: queued messages or transport errors.
// State changes: mutates internal queue state and closed flag.
// Side effects: synchronization across goroutines.
// Compatibility: this keeps the current Go gateway behavior aligned with nanobot's simple in-process message bus.
type MemoryBus struct {
	inbound  chan model.InboundMessage
	outbound chan model.OutboundMessage

	closed chan struct{}

	closeFlag atomic.Bool
	closeOnce sync.Once
}

// NewMemoryBus creates an in-memory bus with the given buffer size.
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

// PublishInbound enqueues an inbound message for the gateway loop.
func (b *MemoryBus) PublishInbound(ctx context.Context, msg model.InboundMessage) error {
	// If the caller already canceled the operation, return that error
	// deterministically instead of letting select randomly pick a ready send.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("publish inbound: %w", err)
	}
	if b.closeFlag.Load() {
		return transport.ErrBusClosed
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("publish inbound: %w", ctx.Err())
	case <-b.closed:
		return transport.ErrBusClosed
	case b.inbound <- msg:
		return nil
	}
}

// ConsumeInbound dequeues the next inbound message for the gateway loop.
func (b *MemoryBus) ConsumeInbound(ctx context.Context) (model.InboundMessage, error) {
	if err := ctx.Err(); err != nil {
		return model.InboundMessage{}, fmt.Errorf("consume inbound: %w", err)
	}
	if b.closeFlag.Load() {
		return model.InboundMessage{}, transport.ErrBusClosed
	}
	select {
	case <-ctx.Done():
		return model.InboundMessage{}, fmt.Errorf("consume inbound: %w", ctx.Err())
	case <-b.closed:
		return model.InboundMessage{}, transport.ErrBusClosed
	case msg := <-b.inbound:
		return msg, nil
	}
}

// PublishOutbound enqueues an outbound message for channel delivery.
func (b *MemoryBus) PublishOutbound(ctx context.Context, msg model.OutboundMessage) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("publish outbound: %w", err)
	}
	if b.closeFlag.Load() {
		return transport.ErrBusClosed
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("publish outbound: %w", ctx.Err())
	case <-b.closed:
		return transport.ErrBusClosed
	case b.outbound <- msg:
		return nil
	}
}

// ConsumeOutbound dequeues the next outbound message for channel delivery.
func (b *MemoryBus) ConsumeOutbound(ctx context.Context) (model.OutboundMessage, error) {
	if err := ctx.Err(); err != nil {
		return model.OutboundMessage{}, fmt.Errorf("consume outbound: %w", err)
	}
	if b.closeFlag.Load() {
		return model.OutboundMessage{}, transport.ErrBusClosed
	}
	select {
	case <-ctx.Done():
		return model.OutboundMessage{}, fmt.Errorf("consume outbound: %w", ctx.Err())
	case <-b.closed:
		return model.OutboundMessage{}, transport.ErrBusClosed
	case msg := <-b.outbound:
		return msg, nil
	}
}

// Close stops the bus and makes future operations return transport.ErrBusClosed.
func (b *MemoryBus) Close() error {
	b.closeOnce.Do(func() {
		b.closeFlag.Store(true)
		close(b.closed)
	})
	return nil
}
