package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"tinybot/internal/domain/model"
	"tinybot/internal/transport"
)

// Loop bridges inbound transport messages to the chat service and republishes outbound replies.
//
// Responsibilities:
//   - consume inbound messages from the transport bus
//   - delegate processing to the chat service
//   - publish either the service reply or a fallback error reply
//
// Inputs: inbound messages from the bus and a MessageProcessor implementation.
// Outputs: outbound messages published back to the bus.
// State changes: none beyond message flow through the bus.
// Side effects: coordinates long-running goroutines via the bus and context.
// Compatibility: preserves the existing Go gateway loop behavior while moving it out of the service/usecase layer.
type Loop struct {
	processor transport.MessageProcessor
	bus       transport.MessageBus
}

// NewLoop creates a gateway loop bound to one processor and one transport bus.
func NewLoop(processor transport.MessageProcessor, bus transport.MessageBus) *Loop {
	return &Loop{
		processor: processor,
		bus:       bus,
	}
}

// Run starts the gateway loop until the context is canceled or the bus closes.
func (l *Loop) Run(ctx context.Context) error {
	for {
		msg, err := l.bus.ConsumeInbound(ctx)
		if err != nil {
			if errors.Is(err, transport.ErrBusClosed) || errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("gateway loop consume inbound: %w", err)
		}

		out, err := l.processor.ProcessMessage(ctx, msg)
		if err != nil {
			fallback := model.OutboundMessage{
				Channel: msg.Channel,
				ChatID:  msg.ChatID,
				ReplyTo: msg.ID,
				Content: fmt.Sprintf("Sorry, I encountered an error: %v", err),
			}
			if pubErr := l.bus.PublishOutbound(ctx, fallback); pubErr != nil {
				if errors.Is(pubErr, transport.ErrBusClosed) || errors.Is(pubErr, context.Canceled) {
					return nil
				}
				return fmt.Errorf("gateway loop publish fallback: %w", pubErr)
			}
			continue
		}

		if strings.TrimSpace(out.Content) == "" {
			continue
		}

		if err := l.bus.PublishOutbound(ctx, out); err != nil {
			if errors.Is(err, transport.ErrBusClosed) || errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("gateway loop publish outbound: %w", err)
		}
	}
}
