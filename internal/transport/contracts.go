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
type MessageProcessor interface {
	ProcessMessage(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error)
}

// StreamingMessageProcessor 支持流式输出的消息处理器
type StreamingMessageProcessor interface {
	// ProcessMessageStream 流式处理消息。
	// onDelta 在每个正文增量到达时被调用。
	// onThinking 在每个推理过程增量到达时被调用（可为 nil）。
	// 返回值是完整的 OutboundMessage，用于记录和后续处理。
	ProcessMessageStream(ctx context.Context, msg model.InboundMessage, onDelta func(delta string), onThinking func(delta string)) (model.OutboundMessage, error)
}

// Channel represents an external chat transport connected to the gateway runtime.
type Channel interface {
	Name() model.Channel
	Start(ctx context.Context) error
	Send(ctx context.Context, out model.OutboundMessage) error
}
