package main

import (
	"context"

	"tinybot/internal/app"
	"tinybot/internal/desktop"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type DesktopApp struct {
	ctx context.Context
	svc *desktop.Service
}

func NewDesktopApp(workspace string) *DesktopApp {
	return &DesktopApp{
		svc: desktop.NewService(workspace),
	}
}

func (a *DesktopApp) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *DesktopApp) Bootstrap() (desktop.AppBootstrap, error) {
	return a.svc.Bootstrap(context.Background())
}

func (a *DesktopApp) ListSessions() ([]desktop.SessionSummary, error) {
	return a.svc.ListSessions(context.Background())
}

func (a *DesktopApp) GetSession(key string) (desktop.SessionDetail, error) {
	return a.svc.GetSession(context.Background(), key)
}

func (a *DesktopApp) CreateSession(req desktop.CreateSessionRequest) (desktop.SessionSummary, error) {
	return a.svc.CreateSession(context.Background(), req)
}

func (a *DesktopApp) RenameSession(key string, title string) (desktop.SessionSummary, error) {
	return a.svc.RenameSession(context.Background(), key, title)
}

func (a *DesktopApp) DeleteSession(key string) error {
	return a.svc.DeleteSession(context.Background(), key)
}

func (a *DesktopApp) GetConfig() (*app.Config, error) {
	return a.svc.GetConfig(context.Background())
}

func (a *DesktopApp) ListProviders() ([]desktop.ProviderInfo, error) {
	return a.svc.ListProviders(context.Background())
}

func (a *DesktopApp) SaveConfig(patch desktop.ConfigPatch) (desktop.AppBootstrap, error) {
	if _, err := a.svc.SaveConfig(context.Background(), patch); err != nil {
		return desktop.AppBootstrap{}, err
	}
	return a.svc.Bootstrap(context.Background())
}

func (a *DesktopApp) SetActiveProvider(name string) (desktop.AppBootstrap, error) {
	if _, err := a.svc.SetActiveProvider(context.Background(), name); err != nil {
		return desktop.AppBootstrap{}, err
	}
	return a.svc.Bootstrap(context.Background())
}

func (a *DesktopApp) StreamMessage(req desktop.SendMessageRequest) (desktop.ChatReply, error) {
	return a.svc.StreamMessage(context.Background(), req, desktop.EventSinkFunc(func(event string, payload any) error {
		if a.ctx == nil {
			return nil
		}
		wailsruntime.EventsEmit(a.ctx, event, payload)
		return nil
	}))
}
