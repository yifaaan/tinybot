package transport

import (
	"context"
	"errors"

	"tinybot/internal/domain/model"
)

// ErrBusClosed reports that a transport message bus is no longer accepting traffic.
//
// Responsibility: provide one shared sentinel error for gateway, bus, and channel code.
// Inputs/outputs: returned by MessageBus operations after shutdown.
// State changes: none.
// Side effects: none.
// Compatibility: preserves the existing Go runtime contract used by the old bus/channel loop.
var ErrBusClosed = errors.New("message bus closed")

// MessageBus defines the in-process transport boundary used by the gateway runtime.
//
// Responsibilities:
//   - accept inbound messages coming from channels
//   - expose inbound messages to the gateway loop
//   - accept outbound messages coming from the chat service
//   - expose outbound messages to channel dispatchers
//
// Inputs: domain inbound/outbound messages plus a context for cancellation.
// Outputs: delivered messages or transport errors such as context cancellation or ErrBusClosed.
// State changes: implementations typically enqueue or dequeue messages.
// Side effects: concrete implementations may block on queues and synchronize goroutines.
// Compatibility: this mirrors nanobot's internal message bus while staying small and idiomatic for Go.
type MessageBus interface {
	PublishInbound(ctx context.Context, msg model.InboundMessage) error
	ConsumeInbound(ctx context.Context) (model.InboundMessage, error)
	PublishOutbound(ctx context.Context, msg model.OutboundMessage) error
	ConsumeOutbound(ctx context.Context) (model.OutboundMessage, error)
	Close() error
}

// MessageProcessor handles one inbound message and returns the outbound reply.
//
// Responsibilities: bridge the gateway runtime to the service layer without leaking gateway details into services.
// Inputs: a domain inbound message.
// Outputs: a domain outbound message or an error.
// State changes: delegated to the concrete service implementation.
// Side effects: delegated to the concrete service implementation, such as LLM calls or session writes.
// Compatibility: this matches the current chat service signature so the gateway loop stays transport-only.
type MessageProcessor interface {
	ProcessMessage(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error)
}

// Channel represents an external chat transport connected to the gateway runtime.
//
// Responsibilities:
//   - read from an external source and publish inbound messages to the bus
//   - send outbound messages back to the external source
//
// Inputs: a context for lifecycle control and outbound messages for delivery.
// Outputs: lifecycle errors from Start or delivery errors from Send.
// State changes: implementation-specific runtime state such as active connections or prompt rendering.
// Side effects: may read stdin, write stdout, or call external APIs/SDKs.
// Compatibility: this aligns with nanobot's channel abstraction while keeping the boundary explicit in Go.
type Channel interface {
	Name() model.Channel
	Start(ctx context.Context) error
	Send(ctx context.Context, out model.OutboundMessage) error
}
