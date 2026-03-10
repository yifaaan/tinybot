package ports

import (
	"context"
	"errors"
	"tinybot/internal/domain/model"
)

var ErrBusClosed = errors.New("message buf closed")

type MessageBus interface {
	PublishInbound(ctx context.Context, msg model.InboundMessage) error
	ConsumeInbound(ctx context.Context) (model.InboundMessage, error)
	PublishOutbound(ctx context.Context, msg model.OutboundMessage) error
	ConsumeOutbound(ctx context.Context) (model.OutboundMessage, error)
	Close() error
}
