package desktop

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"tinybot/internal/app"
	"tinybot/internal/domain/model"
	"tinybot/internal/repository/sessionrepo"
)

type fakeChatRuntime struct {
	processMessage       func(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error)
	processMessageStream func(ctx context.Context, msg model.InboundMessage, onDelta func(string), onThinking func(string)) (model.OutboundMessage, error)
}

func (f fakeChatRuntime) ProcessMessage(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error) {
	return f.processMessage(ctx, msg)
}

func (f fakeChatRuntime) ProcessMessageStream(ctx context.Context, msg model.InboundMessage, onDelta func(string), onThinking func(string)) (model.OutboundMessage, error) {
	return f.processMessageStream(ctx, msg, onDelta, onThinking)
}

func TestService_ListAndGetSession(t *testing.T) {
	workspace := t.TempDir()
	repo := sessionrepo.NewFileSessionRepository(workspace)
	session := model.NewSession("desktop:1")
	session.Metadata["title"] = "Pinned session"
	session.Metadata["provider"] = "qwen"
	session.CreatedAt = time.Date(2026, 3, 17, 8, 0, 0, 0, time.UTC)
	session.UpdatedAt = session.CreatedAt.Add(5 * time.Minute)
	session.AddMessage(model.RoleUser, "hello from desktop", nil)
	session.AddMessage(model.RoleAssistant, "response", nil)
	if err := repo.SaveSession(context.Background(), session); err != nil {
		t.Fatalf("SaveSession() error: %v", err)
	}

	svc := NewService(workspace)
	svc.loadConfig = func(_ string) (*app.Config, error) {
		cfg := app.DefaultConfig()
		cfg.Agents.Workspace = workspace
		return cfg, nil
	}

	list, err := svc.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(ListSessions()) = %d, want 1", len(list))
	}
	if list[0].Title != "Pinned session" {
		t.Fatalf("Title = %q, want %q", list[0].Title, "Pinned session")
	}
	if list[0].Preview != "response" {
		t.Fatalf("Preview = %q, want %q", list[0].Preview, "response")
	}
	if list[0].ProviderName != "qwen" {
		t.Fatalf("ProviderName = %q, want %q", list[0].ProviderName, "qwen")
	}

	detail, err := svc.GetSession(context.Background(), "desktop:1")
	if err != nil {
		t.Fatalf("GetSession() error: %v", err)
	}
	if len(detail.Messages) != 2 {
		t.Fatalf("len(detail.Messages) = %d, want 2", len(detail.Messages))
	}
	if detail.Metadata["title"] != "Pinned session" {
		t.Fatalf("Metadata title = %#v, want %q", detail.Metadata["title"], "Pinned session")
	}
	if detail.Metadata["provider"] != "qwen" {
		t.Fatalf("Metadata provider = %#v, want %q", detail.Metadata["provider"], "qwen")
	}
}

func TestService_CreateRenameDeleteSession(t *testing.T) {
	workspace := t.TempDir()
	svc := NewService(workspace)
	svc.now = func() time.Time {
		return time.Date(2026, 3, 17, 10, 0, 0, 123, time.UTC)
	}
	svc.loadConfig = func(_ string) (*app.Config, error) {
		cfg := app.DefaultConfig()
		cfg.Providers.Active = "openai"
		return cfg, nil
	}

	created, err := svc.CreateSession(context.Background(), CreateSessionRequest{
		Title:        "Fresh chat",
		ProviderName: "qwen",
	})
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}
	if created.Title != "Fresh chat" {
		t.Fatalf("Title = %q, want %q", created.Title, "Fresh chat")
	}
	if created.ProviderName != "qwen" {
		t.Fatalf("ProviderName = %q, want %q", created.ProviderName, "qwen")
	}

	renamed, err := svc.RenameSession(context.Background(), created.Key, "Renamed chat")
	if err != nil {
		t.Fatalf("RenameSession() error: %v", err)
	}
	if renamed.Title != "Renamed chat" {
		t.Fatalf("Title = %q, want %q", renamed.Title, "Renamed chat")
	}

	if err := svc.DeleteSession(context.Background(), created.Key); err != nil {
		t.Fatalf("DeleteSession() error: %v", err)
	}
	repo := sessionrepo.NewFileSessionRepository(workspace)
	if _, err := repo.LoadSession(created.Key); err == nil {
		t.Fatal("expected deleted session to be missing")
	}
}

func TestService_CreateSessionFallsBackToActiveProvider(t *testing.T) {
	workspace := t.TempDir()
	svc := NewService(workspace)
	svc.loadConfig = func(_ string) (*app.Config, error) {
		cfg := app.DefaultConfig()
		cfg.Providers.Active = "openai"
		return cfg, nil
	}

	created, err := svc.CreateSession(context.Background(), CreateSessionRequest{Title: "Fallback provider"})
	if err != nil {
		t.Fatalf("CreateSession() error: %v", err)
	}
	if created.ProviderName != "openai" {
		t.Fatalf("ProviderName = %q, want %q", created.ProviderName, "openai")
	}

	detail, err := svc.GetSession(context.Background(), created.Key)
	if err != nil {
		t.Fatalf("GetSession() error: %v", err)
	}
	if detail.Metadata["provider"] != "openai" {
		t.Fatalf("Metadata provider = %#v, want %q", detail.Metadata["provider"], "openai")
	}
}

func TestService_ListSessionsFallsBackToActiveProviderForLegacySessions(t *testing.T) {
	workspace := t.TempDir()
	repo := sessionrepo.NewFileSessionRepository(workspace)
	session := model.NewSession("desktop:legacy")
	session.Metadata["title"] = "Legacy session"
	if err := repo.SaveSession(context.Background(), session); err != nil {
		t.Fatalf("SaveSession() error: %v", err)
	}

	svc := NewService(workspace)
	svc.loadConfig = func(_ string) (*app.Config, error) {
		cfg := app.DefaultConfig()
		cfg.Providers.Active = "deepseek"
		return cfg, nil
	}

	list, err := svc.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions() error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(ListSessions()) = %d, want 1", len(list))
	}
	if list[0].ProviderName != "deepseek" {
		t.Fatalf("ProviderName = %q, want %q", list[0].ProviderName, "deepseek")
	}

	detail, err := svc.GetSession(context.Background(), session.Key)
	if err != nil {
		t.Fatalf("GetSession() error: %v", err)
	}
	if detail.Metadata["provider"] != "deepseek" {
		t.Fatalf("Metadata provider = %#v, want %q", detail.Metadata["provider"], "deepseek")
	}
}

func TestService_SaveConfigPatchAndProviders(t *testing.T) {
	workspace := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("TINYBOT_CONFIG", configPath)

	cfg := app.DefaultConfig()
	cfg.Agents.Workspace = workspace
	if err := app.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}

	svc := NewService(workspace)
	modelName := "gpt-4.1"
	active := "openai"
	temperature := 0.25
	consoleEnabled := false

	updated, err := svc.SaveConfig(context.Background(), ConfigPatch{
		ActiveProvider: &active,
		Temperature:    &temperature,
		ConsoleEnabled: &consoleEnabled,
		Providers: []ProviderConfigPatch{
			{
				Name:  "openai",
				Model: &modelName,
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}
	if updated.Providers.Active != "openai" {
		t.Fatalf("Providers.Active = %q, want %q", updated.Providers.Active, "openai")
	}
	if updated.Agents.Temperature != 0.25 {
		t.Fatalf("Temperature = %v, want 0.25", updated.Agents.Temperature)
	}
	if updated.Channels.Console.Enabled {
		t.Fatalf("Console.Enabled = true, want false")
	}
	if updated.Providers.List["openai"].Model != "gpt-4.1" {
		t.Fatalf("openai model = %q, want %q", updated.Providers.List["openai"].Model, "gpt-4.1")
	}

	providers, err := svc.ListProviders(context.Background())
	if err != nil {
		t.Fatalf("ListProviders() error: %v", err)
	}
	if len(providers) == 0 {
		t.Fatal("expected providers to be returned")
	}
}

func TestService_StreamMessageEmitsEvents(t *testing.T) {
	workspace := t.TempDir()
	svc := NewService(workspace)
	svc.newRuntime = func(_ string) (chatRuntime, error) {
		return fakeChatRuntime{
			processMessage: func(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error) {
				return model.OutboundMessage{}, nil
			},
			processMessageStream: func(ctx context.Context, msg model.InboundMessage, onDelta func(string), onThinking func(string)) (model.OutboundMessage, error) {
				if msg.SessionKey() != "desktop:abc" {
					t.Fatalf("msg.SessionKey() = %q, want %q", msg.SessionKey(), "desktop:abc")
				}
				onThinking("thinking")
				onDelta("hello ")
				onDelta("world")
				return model.OutboundMessage{Content: "hello world"}, nil
			},
		}, nil
	}

	var events []StreamEvent
	sink := EventSinkFunc(func(event string, payload any) error {
		if event != EventChatStream {
			t.Fatalf("event = %q, want %q", event, EventChatStream)
		}
		item, ok := payload.(StreamEvent)
		if !ok {
			t.Fatalf("payload type = %T, want StreamEvent", payload)
		}
		events = append(events, item)
		return nil
	})

	reply, err := svc.StreamMessage(context.Background(), SendMessageRequest{
		SessionKey: "desktop:abc",
		Content:    "ping",
	}, sink)
	if err != nil {
		t.Fatalf("StreamMessage() error: %v", err)
	}
	if reply.Content != "hello world" {
		t.Fatalf("reply.Content = %q, want %q", reply.Content, "hello world")
	}
	if len(events) != 4 {
		t.Fatalf("len(events) = %d, want 4", len(events))
	}
	if events[0].Kind != "thinking" || events[1].Kind != "delta" || events[3].Kind != "done" {
		t.Fatalf("unexpected event sequence: %#v", events)
	}
}

func TestService_StreamMessageEmitsErrorEvent(t *testing.T) {
	svc := NewService(t.TempDir())
	svc.newRuntime = func(_ string) (chatRuntime, error) {
		return fakeChatRuntime{
			processMessage: func(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error) {
				return model.OutboundMessage{}, nil
			},
			processMessageStream: func(ctx context.Context, msg model.InboundMessage, onDelta func(string), onThinking func(string)) (model.OutboundMessage, error) {
				return model.OutboundMessage{}, errors.New("boom")
			},
		}, nil
	}

	var kinds []string
	sink := EventSinkFunc(func(event string, payload any) error {
		item := payload.(StreamEvent)
		kinds = append(kinds, item.Kind)
		return nil
	})

	_, err := svc.StreamMessage(context.Background(), SendMessageRequest{Content: "ping"}, sink)
	if err == nil {
		t.Fatal("expected stream error")
	}
	if len(kinds) != 1 || kinds[0] != "error" {
		t.Fatalf("kinds = %#v, want [error]", kinds)
	}
}
