package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"tinybot/internal/domain/model"
	"tinybot/internal/transport"
	"tinybot/internal/utils/logger"
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

// streamingProcessor 检测 processor 是否支持流式
type streamingProcessor interface {
	ProcessMessageStream(ctx context.Context, msg model.InboundMessage, onDelta func(string), onThinking func(string)) (model.OutboundMessage, error)
}

// streamWriterChannel 检测 channel 是否支持流式写入
type streamWriterChannel interface {
	WriteDelta(delta string) error
}

// Run starts the gateway loop until the context is canceled or the bus closes.
func (l *Loop) Run(ctx context.Context) error {
	logger.Info("gateway loop started")
	defer logger.Info("gateway loop stopped")

	for {
		msg, err := l.bus.ConsumeInbound(ctx)
		if err != nil {
			if errors.Is(err, transport.ErrBusClosed) || errors.Is(err, context.Canceled) {
				return nil
			}
			logger.Error("consume inbound failed", "error", err)
			return fmt.Errorf("gateway loop consume inbound: %w", err)
		}

		logger.Debug("processing message", "session", msg.SessionKey(), "channel", msg.Channel, "content_len", len(msg.Content))

		var out model.OutboundMessage
		if sp, ok := l.processor.(streamingProcessor); ok && msg.StreamWriter != nil {
			writer := msg.StreamWriter
			var streamed bool

			onDelta := func(delta string) {
				streamed = true
				writer.WriteDelta(delta)
				if flusher, ok := writer.(interface{ Flush() error }); ok {
					flusher.Flush()
				}
			}

			// onThinking：如果 channel 的 StreamWriter 支持 WriteThinking，就转发推理内容
			var onThinking func(string)
			if tw, ok := writer.(model.ThinkingStreamWriter); ok {
				logger.Debug("ThinkingStreamWriter detected", "writer_type", fmt.Sprintf("%T", writer))
				onThinking = func(delta string) {
					tw.WriteThinking(delta)
				}
			}

			out, err = sp.ProcessMessageStream(ctx, msg, onDelta, onThinking)
			out.Streamed = streamed
			logger.Debug("streaming completed", "session", msg.SessionKey(), "streamed", streamed)
		} else {
			out, err = l.processor.ProcessMessage(ctx, msg)
		}

		if err != nil {
			logger.Error("process message failed", "session", msg.SessionKey(), "error", err)
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
				logger.Error("publish fallback failed", "error", pubErr)
				return fmt.Errorf("gateway loop publish fallback: %w", pubErr)
			}
			continue
		}
		if strings.TrimSpace(out.Content) == "" {
			logger.Debug("empty response, skipping", "session", msg.SessionKey())
			continue
		}

		if err := l.bus.PublishOutbound(ctx, out); err != nil {
			if errors.Is(err, transport.ErrBusClosed) || errors.Is(err, context.Canceled) {
				return nil
			}
			logger.Error("publish outbound failed", "error", err)
			return fmt.Errorf("gateway loop publish outbound: %w", err)
		}
		logger.Debug("message completed", "session", msg.SessionKey(), "response_len", len(out.Content))
	}
}
