package app

import (
	"context"
	"errors"
	"fmt"
	"tinybot/internal/adapters/bus"
	"tinybot/internal/usecase/agent"
)

type GatewayApp struct {
	Bus  *bus.MemoryBus
	Loop *agent.Loop
}

func NewGatewayApp(workspace string) (*GatewayApp, error) {
	app, err := NewApp(workspace)
	if err != nil {
		return nil, fmt.Errorf("new gateway app: %w", err)
	}

	bs := bus.NewMemoryBus(16)
	lp := agent.NewLoop(app.ChatUseCase, bs)

	return &GatewayApp{
		Bus:  bs,
		Loop: lp,
	}, nil
}

func (a *GatewayApp) Run(ctx context.Context) error {
	if a == nil || a.Loop == nil || a.Bus == nil {
		return errors.New("gateway app is not properly initialized")
	}
	return a.Loop.Run(ctx)
}

func (a *GatewayApp) Close() error {
	if a == nil || a.Bus == nil {
		return nil
	}
	return a.Bus.Close()
}
