package app

import (
	"context"
	"errors"
	"fmt"
	"io"

	"tinybot/internal/adapters/tool"
	"tinybot/internal/domain/model"
	"tinybot/internal/repository/cronrepo"
	cronservice "tinybot/internal/service/cron"
	heartbeatservice "tinybot/internal/service/heartbeat"
	"tinybot/internal/transport"
	transportbus "tinybot/internal/transport/bus"
	transportchannel "tinybot/internal/transport/channel"
	transportgateway "tinybot/internal/transport/gateway"
	transportruntime "tinybot/internal/transport/runtime"
)

type messageToolRegistry interface {
	SetMessageCallback(callback tool.SendMessageCallback)
}

// wireMessageToolToBus 把 message tool 的发送动作接到 outbound bus 上。
func wireMessageToolToBus(registry messageToolRegistry, bus transport.MessageBus) {
	if registry == nil || bus == nil {
		return
	}
	registry.SetMessageCallback(func(ctx context.Context, msg model.OutboundMessage) error {
		// app 层只负责把 message tool 的“外发意图”重新放回统一的 outbound bus，
		// 真正的发送仍然交给 transport/channel manager。
		return bus.PublishOutbound(ctx, msg)
	})
}

type GatewayApp struct {
	Bus       *transportbus.MemoryBus
	Loop      *transportgateway.Loop
	Manager   *transportchannel.ChannelManager
	Heartbeat *transportruntime.HeartbeatRunner
	Cron      *transportruntime.CronRunner
}

func NewGatewayApp(workspace string, input io.Reader, output io.Writer) (*GatewayApp, error) {
	app, err := NewApp(workspace)
	if err != nil {
		return nil, fmt.Errorf("new gateway app: %w", err)
	}

	bs := transportbus.NewMemoryBus(16)
	app.Tools.Register(tool.NewMessageTool(nil, model.ChannelCLI, ""))
	wireMessageToolToBus(app.Tools, bs)

	lp := transportgateway.NewLoop(app.ChatService, bs)
	manager := transportchannel.NewChannelManager(bs)
	heartbeatSvc := heartbeatservice.NewService(workspace, app.ChatService)
	heartbeatRunner, err := transportruntime.NewHeartbeatRunner(heartbeatSvc, app.Config.Heartbeat.IntervalSeconds, app.Config.Heartbeat.Enabled)
	if err != nil {
		return nil, err
	}

	cronRepo := cronrepo.NewFileCronRepository(workspace)
	cronSvc, err := cronservice.NewService(cronRepo, app.ChatService)
	if err != nil {
		return nil, err
	}
	cronRunner, err := transportruntime.NewCronRunner(cronSvc, 30)
	if err != nil {
		return nil, err
	}
	if app.Config.Channels.Console.Enabled {
		consoleCfg := transportchannel.ConsoleChannelConfig{
			ChatID:     app.Config.Channels.Console.ChatID,
			SenderID:   app.Config.Channels.Console.SenderID,
			Prompt:     app.Config.Channels.Console.Prompt,
			ShowPrefix: app.Config.Channels.Console.ShowPrefix,
		}
		consoleCh := transportchannel.NewConsoleChannel(bs, consoleCfg, input, output)
		manager.RegisterChannel(consoleCh)
	}
	return &GatewayApp{
		Bus:       bs,
		Loop:      lp,
		Manager:   manager,
		Heartbeat: heartbeatRunner,
		Cron:      cronRunner,
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
