package channel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"tinybot/internal/domain/model"
	"tinybot/internal/ports"
)

// ChannelManager manages multiple channels and routes outbound messages between them and the message bus.
// Responsibilities:
//
//	启动每个Channel
//	从MessageBus消费OutboundMessage，并根据Channel字段路由到对应的Channel发送
type ChannelManager struct {
	bus      ports.MessageBus
	channels map[model.Channel]ports.Channel
}

func NewChannelManager(bus ports.MessageBus) *ChannelManager {
	return &ChannelManager{
		bus:      bus,
		channels: make(map[model.Channel]ports.Channel),
	}
}

func (m *ChannelManager) RegisterChannel(ch ports.Channel) {
	m.channels[ch.Name()] = ch
}

func (m *ChannelManager) Start(ctx context.Context) error {
	if len(m.channels) == 0 {
		return errors.New("channel manager: no channels registered")
	}

	errCh := make(chan error, len(m.channels)+1)
	// Start each channel in a separate goroutine
	var wg sync.WaitGroup
	for _, ch := range m.channels {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if err := ch.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("channel %q start: %w", ch.Name(), err)
			}
		}()
	}

	// Start a goroutine to consume outbound messages and route them to channels
	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := m.dispatchOutbound(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- fmt.Errorf("dispatch outbound: %w", err)
		}
	}()

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	case <-done:
		return nil
	}
}

func (m *ChannelManager) dispatchOutbound(ctx context.Context) error {
	for {
		msg, err := m.bus.ConsumeOutbound(ctx)
		if err != nil {
			if errors.Is(err, ports.ErrBusClosed) || errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("consume outbound: %w", err)
		}

		ch, ok := m.channels[msg.Channel]
		if !ok {
			continue
		}

		if err := ch.Send(ctx, msg); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("send message: %w", err)
		}
	}
}
