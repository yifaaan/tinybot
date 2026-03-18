package desktop

import (
	"time"

	"tinybot/internal/app"
)

const EventChatStream = "desktop:chat-stream"

type AppBootstrap struct {
	Workspace string           `json:"workspace"`
	Status    app.Status       `json:"status"`
	Config    *app.Config      `json:"config"`
	Providers []ProviderInfo   `json:"providers"`
	Sessions  []SessionSummary `json:"sessions"`
}

type ProviderInfo struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Model      string `json:"model"`
	APIBase    string `json:"apiBase"`
	Active     bool   `json:"active"`
	Configured bool   `json:"configured"`
}

type SessionSummary struct {
	Key          string    `json:"key"`
	Title        string    `json:"title"`
	Preview      string    `json:"preview"`
	ProviderName string    `json:"providerName"`
	Channel      string    `json:"channel"`
	MessageCount int       `json:"messageCount"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type SessionMessage struct {
	Role       string    `json:"role"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"createdAt"`
	Thinking   string    `json:"thinking,omitempty"`
	Name       string    `json:"name,omitempty"`
	ToolCallID string    `json:"toolCallId,omitempty"`
}

type SessionDetail struct {
	Summary  SessionSummary   `json:"summary"`
	Messages []SessionMessage `json:"messages"`
	Metadata map[string]any   `json:"metadata"`
}

type CreateSessionRequest struct {
	Title        string `json:"title"`
	ProviderName string `json:"providerName"`
}

type SendMessageRequest struct {
	SessionKey     string           `json:"sessionKey"`
	Content        string           `json:"content"`
	SelectedSkills []string         `json:"selectedSkills,omitempty"`
	Attachments    []FileAttachment `json:"attachments,omitempty"`
}

type FileAttachment struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Size    int64  `json:"size"`
	Preview string `json:"preview,omitempty"`
	Content string `json:"content,omitempty"` // For text files
	Path    string `json:"path,omitempty"`
}

type ChatReply struct {
	SessionKey string    `json:"sessionKey"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"createdAt"`
}

type StreamEvent struct {
	SessionKey string `json:"sessionKey"`
	Kind       string `json:"kind"`
	Delta      string `json:"delta,omitempty"`
	Content    string `json:"content,omitempty"`
	Error      string `json:"error,omitempty"`
}

type ConfigPatch struct {
	Workspace                *string               `json:"workspace,omitempty"`
	ActiveProvider           *string               `json:"activeProvider,omitempty"`
	MaxTokens                *int                  `json:"maxTokens,omitempty"`
	Temperature              *float64              `json:"temperature,omitempty"`
	MaxToolIterations        *int                  `json:"maxToolIterations,omitempty"`
	EnableThinking           *bool                 `json:"enableThinking,omitempty"`
	ReasoningEffort          *string               `json:"reasoningEffort,omitempty"`
	ReasoningSummary         *string               `json:"reasoningSummary,omitempty"`
	TextVerbosity            *string               `json:"textVerbosity,omitempty"`
	HeartbeatEnabled         *bool                 `json:"heartbeatEnabled,omitempty"`
	HeartbeatIntervalSeconds *int                  `json:"heartbeatIntervalSeconds,omitempty"`
	LogLevel                 *string               `json:"logLevel,omitempty"`
	LogFormat                *string               `json:"logFormat,omitempty"`
	LogOutput                *string               `json:"logOutput,omitempty"`
	ConsoleEnabled           *bool                 `json:"consoleEnabled,omitempty"`
	ConsolePrompt            *string               `json:"consolePrompt,omitempty"`
	ConsoleShowPrefix        *bool                 `json:"consoleShowPrefix,omitempty"`
	TelegramEnabled          *bool                 `json:"telegramEnabled,omitempty"`
	TelegramToken            *string               `json:"telegramToken,omitempty"`
	Providers                []ProviderConfigPatch `json:"providers,omitempty"`
}

type ProviderConfigPatch struct {
	Name    string  `json:"name"`
	Kind    *string `json:"kind,omitempty"`
	Model   *string `json:"model,omitempty"`
	APIKey  *string `json:"apiKey,omitempty"`
	APIBase *string `json:"apiBase,omitempty"`
}

type EventSink interface {
	Emit(event string, payload any) error
}

type EventSinkFunc func(event string, payload any) error

func (f EventSinkFunc) Emit(event string, payload any) error {
	return f(event, payload)
}
