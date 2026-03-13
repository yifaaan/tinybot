package chat

import (
	"context"

	"tinybot/internal/domain/model"
)

// SessionRepository stores and retrieves chat sessions for the service layer.
//
// Responsibilities:
//   - load or create a session for a message thread
//   - persist the updated session trace after each turn
//
// Inputs: a stable session key derived from the inbound message or an explicit override.
// Outputs: a mutable domain session and persistence errors when saving fails.
// State changes: session contents grow as the chat service appends user/tool/assistant trace items.
// Side effects: concrete implementations usually read and write JSONL files.
// Compatibility: this keeps the current Go JSONL session behavior aligned with nanobot's file-based session manager.
type SessionRepository interface {
	GetOrCreateSession(ctx context.Context, key string) (*model.Session, error)
	SaveSession(ctx context.Context, session *model.Session) error
}

// CompletionClient is the service-facing LLM boundary.
//
// Inputs: assembled LLM messages, tool schemas, token limit, and temperature.
// Outputs: a normalized LLMResponse containing assistant text and optional tool calls.
// State changes: none inside the interface itself.
// Side effects: concrete adapters typically make network requests to a model provider.
// Compatibility: this mirrors nanobot's provider abstraction without leaking provider-specific types into the service layer.
type CompletionClient interface {
	Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error)
}

// ToolExecutor exposes registered tools to the chat service.
//
// Inputs: a tool name and JSON-like arguments chosen by the model.
// Outputs: tool definitions for schema exposure and string results for tool execution.
// State changes: tool implementations may mutate files, execute commands, or call external services.
// Side effects: defined by each concrete tool implementation.
// Compatibility: this preserves the registry-driven tool loop used by the current Go code and by nanobot.
type ToolExecutor interface {
	GetDefinitions() []map[string]any
	Execute(ctx context.Context, name string, params map[string]any) (string, error)
}

// PromptBuilder assembles the message list that is sent to the completion client.
//
// Inputs: prior history, the current user message, and optional skill names.
// Outputs: LLM-ready messages plus helper transforms for assistant/tool trace items.
// State changes: none inside the interface itself.
// Side effects: concrete builders may read workspace files, memory notes, and skill documents.
// Compatibility: this captures nanobot's context-builder boundary while keeping orchestration inside the service.
type PromptBuilder interface {
	BuildMessages(history []*model.Message, currentMessage string, skillNames []string) []map[string]any
	AddAssistantMessage(messages []map[string]any, content string, toolCalls []map[string]any) []map[string]any
	AddToolResult(messages []map[string]any, toolCallID string, toolName string, result string) []map[string]any
}
