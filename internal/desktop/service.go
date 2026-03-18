package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"tinybot/internal/app"
	"tinybot/internal/domain/model"
	"tinybot/internal/repository/sessionrepo"
)

const sessionProviderMetadataKey = "provider"

type chatRuntime interface {
	ProcessMessage(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error)
	ProcessMessageStream(ctx context.Context, msg model.InboundMessage, onDelta func(string), onThinking func(string)) (model.OutboundMessage, error)
}

type Service struct {
	workspace      string
	loadConfig     func(workspace string) (*app.Config, error)
	saveConfig     func(cfg *app.Config) error
	checkStatus    func(workspace string) app.Status
	newRuntime     func(workspace string) (chatRuntime, error)
	newSessionRepo func(workspace string) *sessionrepo.FileSessionRepository
	now            func() time.Time
}

func NewService(workspace string) *Service {
	return &Service{
		workspace:      app.ResolveWorkspacePath(workspace),
		loadConfig:     app.LoadConfig,
		saveConfig:     app.SaveConfig,
		checkStatus:    app.CheckStatus,
		newRuntime:     defaultRuntime,
		newSessionRepo: sessionrepo.NewFileSessionRepository,
		now:            time.Now,
	}
}

func defaultRuntime(workspace string) (chatRuntime, error) {
	appInstance, err := app.NewApp(workspace)
	if err != nil {
		return nil, err
	}
	return appInstance.ChatService, nil
}

func (s *Service) Bootstrap(ctx context.Context) (AppBootstrap, error) {
	if err := ctx.Err(); err != nil {
		return AppBootstrap{}, fmt.Errorf("bootstrap: %w", err)
	}

	cfg, err := s.loadConfig(s.workspace)
	if err != nil {
		return AppBootstrap{}, fmt.Errorf("bootstrap load config: %w", err)
	}
	sessions, err := s.ListSessions(ctx)
	if err != nil {
		return AppBootstrap{}, err
	}

	return AppBootstrap{
		Workspace: s.workspace,
		Status:    s.checkStatus(s.workspace),
		Config:    cfg,
		Providers: listProvidersFromConfig(cfg),
		Sessions:  sessions,
	}, nil
}

func (s *Service) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	fallbackProvider := s.activeProviderName()
	repo := s.newSessionRepo(s.workspace)
	list, err := repo.ListSessions()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []SessionSummary{}, nil
		}
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	summaries := make([]SessionSummary, 0, len(list))
	for _, item := range list {
		key, _ := item["key"].(string)
		metadata, _ := item["metadata"].(map[string]any)
		messageCount, _ := item["message_count"].(int)
		firstUserContent, _ := item["first_user_content"].(string)

		// Get title from metadata or generate from first user message
		title := strings.TrimSpace(asString(metadata["title"]))
		if title == "" && firstUserContent != "" {
			title = truncate(firstUserContent, 48)
		}
		if title == "" {
			title = key
		}

		// Get provider name from metadata or use fallback
		providerName := sessionProviderName(metadata, fallbackProvider)

		summaries = append(summaries, SessionSummary{
			Key:          key,
			Title:        title,
			Preview:      truncate(firstUserContent, 120),
			ProviderName: providerName,
			Channel:      sessionChannel(key),
			MessageCount: messageCount,
			CreatedAt:    parseMapTime(item, "created_at"),
			UpdatedAt:    parseMapTime(item, "updated_at"),
		})
	}
	return summaries, nil
}

func (s *Service) GetSession(ctx context.Context, key string) (SessionDetail, error) {
	if err := ctx.Err(); err != nil {
		return SessionDetail{}, fmt.Errorf("get session: %w", err)
	}

	repo := s.newSessionRepo(s.workspace)
	session, err := repo.LoadSession(strings.TrimSpace(key))
	if err != nil {
		return SessionDetail{}, fmt.Errorf("get session: %w", err)
	}
	fallbackProvider := s.activeProviderName()
	metadata := cloneMap(session.Metadata)
	if effectiveProvider := sessionProviderName(metadata, fallbackProvider); effectiveProvider != "" {
		metadata[sessionProviderMetadataKey] = effectiveProvider
	}

	detail := SessionDetail{
		Summary:  buildSessionSummary(session, fallbackProvider),
		Metadata: metadata,
		Messages: make([]SessionMessage, 0, len(session.Messages)),
	}
	for _, msg := range session.Messages {
		if msg == nil {
			continue
		}
		detail.Messages = append(detail.Messages, SessionMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			CreatedAt:  msg.CreatedAt,
			Thinking:   msg.Thinking,
			Name:       msg.Name,
			ToolCallID: msg.ToolCallID,
		})
	}
	return detail, nil
}

func (s *Service) CreateSession(ctx context.Context, req CreateSessionRequest) (SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return SessionSummary{}, fmt.Errorf("create session: %w", err)
	}

	now := s.now()
	key := fmt.Sprintf("%s:%d", model.ChannelDesktop, now.UnixNano())
	session := model.NewSession(key)
	session.CreatedAt = now
	session.UpdatedAt = now
	if title := strings.TrimSpace(req.Title); title != "" {
		session.Metadata["title"] = title
	}
	providerName := strings.TrimSpace(req.ProviderName)
	if providerName == "" {
		providerName = s.activeProviderName()
	}
	if providerName != "" {
		session.Metadata[sessionProviderMetadataKey] = providerName
	}

	repo := s.newSessionRepo(s.workspace)
	if err := repo.SaveSession(ctx, session); err != nil {
		return SessionSummary{}, fmt.Errorf("create session: %w", err)
	}
	return buildSessionSummary(session, providerName), nil
}

func (s *Service) RenameSession(ctx context.Context, key string, title string) (SessionSummary, error) {
	repo := s.newSessionRepo(s.workspace)
	session, err := repo.RenameSession(ctx, key, title)
	if err != nil {
		return SessionSummary{}, fmt.Errorf("rename session: %w", err)
	}
	return buildSessionSummary(session, s.activeProviderName()), nil
}

func (s *Service) DeleteSession(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	repo := s.newSessionRepo(s.workspace)
	return repo.DeleteSession(key)
}

func (s *Service) GetConfig(ctx context.Context) (*app.Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}
	cfg, err := s.loadConfig(s.workspace)
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}
	return cfg, nil
}

func (s *Service) SaveConfig(ctx context.Context, patch ConfigPatch) (*app.Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	cfg, err := s.loadConfig(s.workspace)
	if err != nil {
		return nil, fmt.Errorf("save config load: %w", err)
	}

	applyConfigPatch(cfg, patch)
	if cfg.Providers.List == nil {
		cfg.Providers.List = make(map[string]app.ProviderEntry)
	}
	if strings.TrimSpace(cfg.Providers.Active) != "" {
		if _, ok := cfg.Providers.List[cfg.Providers.Active]; !ok {
			return nil, fmt.Errorf("save config: active provider %q not found", cfg.Providers.Active)
		}
	}

	if err := s.saveConfig(cfg); err != nil {
		return nil, fmt.Errorf("save config write: %w", err)
	}
	return cfg, nil
}

func (s *Service) SetActiveProvider(ctx context.Context, name string) (*app.Config, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("set active provider: name is required")
	}
	return s.SaveConfig(ctx, ConfigPatch{ActiveProvider: &name})
}

func (s *Service) ListProviders(ctx context.Context) ([]ProviderInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	cfg, err := s.loadConfig(s.workspace)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	return listProvidersFromConfig(cfg), nil
}

func (s *Service) SendMessage(ctx context.Context, req SendMessageRequest) (ChatReply, error) {
	runtime, err := s.newRuntime(s.workspace)
	if err != nil {
		return ChatReply{}, fmt.Errorf("send message runtime: %w", err)
	}
	reply, err := runtime.ProcessMessage(ctx, s.buildInboundMessage(req))
	if err != nil {
		return ChatReply{}, fmt.Errorf("send message: %w", err)
	}
	return ChatReply{
		SessionKey: sessionKeyOrDefault(req.SessionKey),
		Content:    reply.Content,
		CreatedAt:  s.now(),
	}, nil
}

func (s *Service) StreamMessage(ctx context.Context, req SendMessageRequest, sink EventSink) (ChatReply, error) {
	runtime, err := s.newRuntime(s.workspace)
	if err != nil {
		return ChatReply{}, fmt.Errorf("stream message runtime: %w", err)
	}

	key := sessionKeyOrDefault(req.SessionKey)
	emit := func(kind string, update StreamEvent) error {
		if sink == nil {
			return nil
		}
		update.SessionKey = key
		update.Kind = kind
		return sink.Emit(EventChatStream, update)
	}

	reply, err := runtime.ProcessMessageStream(ctx, s.buildInboundMessage(req), func(delta string) {
		_ = emit("delta", StreamEvent{Delta: delta})
	}, func(delta string) {
		_ = emit("thinking", StreamEvent{Delta: delta})
	})
	if err != nil {
		_ = emit("error", StreamEvent{Error: err.Error()})
		return ChatReply{}, fmt.Errorf("stream message: %w", err)
	}

	out := ChatReply{
		SessionKey: key,
		Content:    reply.Content,
		CreatedAt:  s.now(),
	}
	if err := emit("done", StreamEvent{Content: out.Content}); err != nil {
		return ChatReply{}, fmt.Errorf("stream message emit: %w", err)
	}
	return out, nil
}

func (s *Service) RetryMessage(ctx context.Context, sessionKey string, sink EventSink) (ChatReply, error) {
	key := sessionKeyOrDefault(sessionKey)
	repo := s.newSessionRepo(s.workspace)

	session, err := repo.LoadSession(key)
	if err != nil {
		return ChatReply{}, fmt.Errorf("retry message load session: %w", err)
	}

	var lastUserContent string
	var messagesToRemove int
	for i := len(session.Messages) - 1; i >= 0; i-- {
		msg := session.Messages[i]
		if msg == nil {
			continue
		}
		if msg.Role == model.RoleUser {
			lastUserContent = strings.TrimSpace(msg.Content)
			messagesToRemove = len(session.Messages) - i - 1
			break
		}
	}

	if lastUserContent == "" {
		return ChatReply{}, errors.New("retry message: no user message found")
	}

	session.Messages = session.Messages[:len(session.Messages)-messagesToRemove]
	session.UpdatedAt = s.now()
	if err := repo.SaveSession(ctx, session); err != nil {
		return ChatReply{}, fmt.Errorf("retry message save session: %w", err)
	}

	return s.StreamMessage(ctx, SendMessageRequest{
		SessionKey: key,
		Content:    lastUserContent,
	}, sink)
}

func (s *Service) buildInboundMessage(req SendMessageRequest) model.InboundMessage {
	key := sessionKeyOrDefault(req.SessionKey)
	now := s.now()

	// Extract image previews (base64 data URLs) for multimodal support
	var mediaURLs []string
	var textAttachments []string
	for _, att := range req.Attachments {
		if att.Preview != "" && strings.HasPrefix(att.Preview, "data:image/") {
			mediaURLs = append(mediaURLs, att.Preview)
		}
		// For text files, include content in the message
		if att.Content != "" {
			textAttachments = append(textAttachments, fmt.Sprintf("### %s\n\n%s", att.Name, att.Content))
		}
	}

	// Build content with text attachments
	content := strings.TrimSpace(req.Content)
	if len(textAttachments) > 0 {
		content = content + "\n\n---\n\n" + strings.Join(textAttachments, "\n\n---\n\n")
	}

	return model.InboundMessage{
		ID:             fmt.Sprintf("desktop-%d", now.UnixNano()),
		Channel:        model.ChannelDesktop,
		SenderID:       "desktop-user",
		ChatID:         "desktop",
		Content:        content,
		MediaURLs:      mediaURLs,
		SelectedSkills: append([]string(nil), req.SelectedSkills...),
		CreatedAt:      now,
		SessionKeyOverride: func() *string {
			sessionKey := key
			return &sessionKey
		}(),
	}
}

func listProvidersFromConfig(cfg *app.Config) []ProviderInfo {
	if cfg == nil {
		return nil
	}

	names := make([]string, 0, len(cfg.Providers.List))
	for name := range cfg.Providers.List {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]ProviderInfo, 0, len(names))
	for _, name := range names {
		entry := cfg.Providers.List[name]
		out = append(out, ProviderInfo{
			Name:       name,
			Kind:       entry.Kind,
			Model:      entry.Model,
			APIBase:    entry.ApiBase,
			Active:     name == cfg.Providers.Active,
			Configured: strings.TrimSpace(entry.ApiKey) != "" || strings.HasPrefix(strings.ToLower(strings.TrimSpace(entry.ApiBase)), "http://localhost"),
		})
	}
	return out
}

func applyConfigPatch(cfg *app.Config, patch ConfigPatch) {
	if patch.Workspace != nil {
		cfg.Agents.Workspace = strings.TrimSpace(*patch.Workspace)
	}
	if patch.ActiveProvider != nil {
		cfg.Providers.Active = strings.TrimSpace(*patch.ActiveProvider)
	}
	if patch.MaxTokens != nil {
		cfg.Agents.MaxTokens = *patch.MaxTokens
	}
	if patch.Temperature != nil {
		cfg.Agents.Temperature = *patch.Temperature
	}
	if patch.MaxToolIterations != nil {
		cfg.Agents.MaxToolIterations = *patch.MaxToolIterations
	}
	if patch.EnableThinking != nil {
		cfg.Agents.EnableThinking = *patch.EnableThinking
	}
	if patch.ReasoningEffort != nil {
		cfg.Agents.ReasoningEffort = strings.TrimSpace(*patch.ReasoningEffort)
	}
	if patch.ReasoningSummary != nil {
		cfg.Agents.ReasoningSummary = strings.TrimSpace(*patch.ReasoningSummary)
	}
	if patch.TextVerbosity != nil {
		cfg.Agents.TextVerbosity = strings.TrimSpace(*patch.TextVerbosity)
	}
	if patch.HeartbeatEnabled != nil {
		cfg.Heartbeat.Enabled = *patch.HeartbeatEnabled
	}
	if patch.HeartbeatIntervalSeconds != nil {
		cfg.Heartbeat.IntervalSeconds = *patch.HeartbeatIntervalSeconds
	}
	if patch.LogLevel != nil {
		cfg.Log.Level = strings.TrimSpace(*patch.LogLevel)
	}
	if patch.LogFormat != nil {
		cfg.Log.Format = strings.TrimSpace(*patch.LogFormat)
	}
	if patch.LogOutput != nil {
		cfg.Log.Output = strings.TrimSpace(*patch.LogOutput)
	}
	if patch.ConsoleEnabled != nil {
		cfg.Channels.Console.Enabled = *patch.ConsoleEnabled
	}
	if patch.ConsolePrompt != nil {
		cfg.Channels.Console.Prompt = *patch.ConsolePrompt
	}
	if patch.ConsoleShowPrefix != nil {
		cfg.Channels.Console.ShowPrefix = *patch.ConsoleShowPrefix
	}
	if patch.TelegramEnabled != nil {
		cfg.Channels.Telegram.Enabled = *patch.TelegramEnabled
	}
	if patch.TelegramToken != nil {
		cfg.Channels.Telegram.Token = strings.TrimSpace(*patch.TelegramToken)
	}
	if cfg.Providers.List == nil {
		cfg.Providers.List = make(map[string]app.ProviderEntry)
	}
	for _, provider := range patch.Providers {
		name := strings.TrimSpace(provider.Name)
		if name == "" {
			continue
		}
		entry := cfg.Providers.List[name]
		if provider.Kind != nil {
			entry.Kind = strings.TrimSpace(*provider.Kind)
		}
		if provider.Model != nil {
			entry.Model = strings.TrimSpace(*provider.Model)
		}
		if provider.APIKey != nil {
			entry.ApiKey = strings.TrimSpace(*provider.APIKey)
		}
		if provider.APIBase != nil {
			entry.ApiBase = strings.TrimSpace(*provider.APIBase)
		}
		if entry.Kind == "" {
			entry.Kind = name
		}
		cfg.Providers.List[name] = entry
	}
}

func buildSessionSummary(session *model.Session, fallbackProvider string) SessionSummary {
	if session == nil {
		return SessionSummary{}
	}

	title := strings.TrimSpace(asString(session.Metadata["title"]))
	providerName := sessionProviderName(session.Metadata, fallbackProvider)
	firstUser := ""
	lastVisible := ""
	for _, msg := range session.Messages {
		if msg == nil {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if msg.Role == model.RoleUser && firstUser == "" {
			firstUser = content
		}
		lastVisible = content
	}
	if title == "" {
		switch {
		case firstUser != "":
			title = truncate(firstUser, 48)
		case lastVisible != "":
			title = truncate(lastVisible, 48)
		default:
			title = session.Key
		}
	}

	return SessionSummary{
		Key:          session.Key,
		Title:        title,
		Preview:      truncate(lastVisible, 120),
		ProviderName: providerName,
		Channel:      sessionChannel(session.Key),
		MessageCount: len(session.Messages),
		CreatedAt:    session.CreatedAt,
		UpdatedAt:    session.UpdatedAt,
	}
}

func (s *Service) activeProviderName() string {
	cfg, err := s.loadConfig(s.workspace)
	if err != nil || cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Providers.Active)
}

func parseMapTime(data map[string]any, key string) time.Time {
	if data == nil {
		return time.Time{}
	}
	raw, _ := data[key].(string)
	t, _ := time.Parse(time.RFC3339, raw)
	return t
}

func sessionChannel(key string) string {
	prefix, _, ok := strings.Cut(key, ":")
	if !ok {
		return ""
	}
	return prefix
}

func sessionKeyOrDefault(key string) string {
	key = strings.TrimSpace(key)
	if key != "" {
		return key
	}
	return fmt.Sprintf("%s:default", model.ChannelDesktop)
}

func truncate(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 || len(text) <= limit {
		return text
	}
	if limit <= 1 {
		return text[:limit]
	}
	return text[:limit-1] + "..."
}

func asString(value any) string {
	s, _ := value.(string)
	return s
}

func sessionProviderName(metadata map[string]any, fallbackProvider string) string {
	providerName := strings.TrimSpace(asString(metadata[sessionProviderMetadataKey]))
	if providerName != "" {
		return providerName
	}
	return strings.TrimSpace(fallbackProvider)
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
