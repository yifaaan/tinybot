package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"tinybot/internal/domain/model"
	"tinybot/internal/ports"
)

type MessageProcessor interface {
	ProcessMessage(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error)
}

// Loop 负责把“总线消息”接到现有 chat use case 上。
//
// 它的责任边界：
// - 它负责消费 inbound
// - 它负责调用 processor
// - 它负责发布 outbound
// - 它不负责 session 持久化细节
// - 它不负责 LLM 细节
// - 它不负责 Telegram / WhatsApp 细节
type Loop struct {
	processor MessageProcessor
	bus       ports.MessageBus
}

func NewLoop(processor MessageProcessor, bus ports.MessageBus) *Loop {
	return &Loop{
		processor: processor,
		bus:       bus,
	}
}

func (l *Loop) Run(ctx context.Context) error {
	for {
		// Get message
		msg, err := l.bus.ConsumeInbound(ctx)
		if err != nil {
			if errors.Is(err, ports.ErrBusClosed) || errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("agent loop consume inbound: %w", err)
		}

		// Process message, get response
		out, err := l.processor.ProcessMessage(ctx, msg)
		if err != nil {
			fallback := model.OutboundMessage{
				Channel: msg.Channel,
				ChatID:  msg.ChatID,
				ReplyTo: msg.ID,
				Content: fmt.Sprintf("Sorry, I encountered an error: %v", err),
			}
			if pubErr := l.bus.PublishOutbound(ctx, fallback); pubErr != nil {
				if errors.Is(pubErr, ports.ErrBusClosed) || errors.Is(pubErr, context.Canceled) {
					return nil
				}
				return fmt.Errorf("agent loop publish fallback: %w", pubErr)
			}
			continue
		}

		if strings.TrimSpace(out.Content) == "" {
			continue
		}

		// Send response to bus
		if err := l.bus.PublishOutbound(ctx, out); err != nil {
			if errors.Is(err, ports.ErrBusClosed) || errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("agent loop publish outbound: %w", err)
		}
	}
}
