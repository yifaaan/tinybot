package ports

import (
	"context"
	"tinybot/internal/domain/model"
)

type Channel interface {
	Name() model.Channel
	// Waiting for receiving message from stdin / HTTP / SDK
	// Construct model.InboundMessage and push it into MessageBus
	Start(ctx context.Context) error
	// Send the response from agent to Channel
	Send(ctx context.Context, out model.OutboundMessage) error
}
