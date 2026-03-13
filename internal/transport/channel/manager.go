package channel

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"tinybot/internal/domain/model"
	"tinybot/internal/transport"
)

// ChannelManager starts registered channels and routes outbound bus traffic to them.
//
// Responsibilities:
//   - keep a registry of enabled channels
//   - start each channel with the shared runtime context
//   - consume outbound messages from the bus and forward them to the matching channel
//
// Inputs: a transport bus plus registered channels.
// Outputs: lifecycle or dispatch errors.
// State changes: channel registry contents and in-flight goroutine lifecycle.
// Side effects: starts goroutines and triggers external channel Send/Start calls.
// Compatibility: mirrors nanobot's channel manager while staying explicit about the transport-only scope.
type ChannelManager struct {
	bus      transport.MessageBus
	channels map[model.Channel]transport.Channel
}

// NewChannelManager builds a transport channel manager around one bus.
func NewChannelManager(bus transport.MessageBus) *ChannelManager {
	return &ChannelManager{
		bus:      bus,
		channels: make(map[model.Channel]transport.Channel),
	}
}

// RegisterChannel adds one channel to the manager registry.
func (m *ChannelManager) RegisterChannel(ch transport.Channel) {
	m.channels[ch.Name()] = ch
}

// Start runs all registered channels and the outbound dispatcher until completion or cancellation.
func (m *ChannelManager) Start(ctx context.Context) error {
	if len(m.channels) == 0 {
		return errors.New("channel manager: no channels registered")
	}

	errCh := make(chan error, len(m.channels)+1)
	var wg sync.WaitGroup

	for _, ch := range m.channels {
		channelRef := ch
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := channelRef.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("channel %q start: %w", channelRef.Name(), err)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := m.dispatchOutbound(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- fmt.Errorf("dispatch outbound: %w", err)
		}
	}()

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
			if errors.Is(err, transport.ErrBusClosed) || errors.Is(err, context.Canceled) {
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
