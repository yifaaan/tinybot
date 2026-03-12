package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"tinybot/internal/adapters/bus"
	"tinybot/internal/adapters/channel"
	"tinybot/internal/adapters/repository"

	"tinybot/internal/usecase/agent"
	"tinybot/internal/usecase/cron"
	"tinybot/internal/usecase/heartbeat"
)

type GatewayApp struct {
	Bus       *bus.MemoryBus
	Loop      *agent.Loop
	Manager   *channel.ChannelManager
	Heartbeat *heartbeat.Service
	Cron      *cron.Service
}

func NewGatewayApp(workspace string, input io.Reader, output io.Writer) (*GatewayApp, error) {
	app, err := NewApp(workspace)
	if err != nil {
		return nil, fmt.Errorf("new gateway app: %w", err)
	}

	bs := bus.NewMemoryBus(16)
	lp := agent.NewLoop(app.ChatUseCase, bs)
	manager := channel.NewChannelManager(bs)
	heartbeatSvc := heartbeat.NewService(workspace, app.ChatUseCase, app.Config.Heartbeat.IntervalSeconds, app.Config.Heartbeat.Enabled)

	cronRepo := repository.NewFileCronRepository(workspace)
	cronSvc, err := cron.NewService(cronRepo, app.ChatUseCase, 30)
	if err != nil {
		return nil, err
	}
	// 根据配置注册 ConsoleChannel
	if app.Config.Channels.Console.Enabled {
		consoleCfg := channel.ConsoleChannelConfig{
			ChatID:     app.Config.Channels.Console.ChatID,
			SenderID:   app.Config.Channels.Console.SenderID,
			Prompt:     app.Config.Channels.Console.Prompt,
			ShowPrefix: app.Config.Channels.Console.ShowPrefix,
		}
		consoleCh := channel.NewConsoleChannel(bs, consoleCfg, input, output)
		manager.RegisterChannel(consoleCh)
	}
	return &GatewayApp{
		Bus:       bs,
		Loop:      lp,
		Manager:   manager,
		Heartbeat: heartbeatSvc,
		Cron:      cronSvc,
	}, nil
}

func (a *GatewayApp) Run(ctx context.Context) error {
	if a == nil || a.Loop == nil || a.Bus == nil {
		return errors.New("gateway app is not properly initialized")
	}

	errCh := make(chan error, 4)

	go func() {
		errCh <- a.Loop.Run(ctx)
	}()

	go func() {
		errCh <- a.Manager.Start(ctx)
	}()

	go func() {
		errCh <- a.Heartbeat.Run(ctx)
	}()

	go func() {
		errCh <- a.Cron.Run(ctx)
	}()

	for i := 0; i < 4; i++ {
		if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("gateway app run: %w", err)
		}
	}
	return nil
}

func (a *GatewayApp) Close() error {
	if a == nil || a.Bus == nil {
		return nil
	}
	return a.Bus.Close()
}
