package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"tinybot/internal/domain/model"
	"tinybot/internal/ports"
)

// UseCase is the use case for the chat.
// It is the core processing engine.
//
// It:
//
// 1. Receives messages from the bus
//
// 2. Builds context with history, memory, skills
//
// 3. Calls the LLM
//
// 4. Executes tool calls
//
// 5. Sends responses back
type UseCase struct {
	sessionRepo ports.SessionRepository
	llmClient   ports.LLMClient
}

// NewUseCase creates a new chat use case.
// It returns an error if the session repository or llm client is nil.
func NewUseCase(sessionRepo ports.SessionRepository, llmClient ports.LLMClient) (*UseCase, error) {
	if sessionRepo == nil {
		return nil, errors.New("chat usecase: session repository is required")
	}
	if llmClient == nil {
		return nil, errors.New("chat usecase: llm client is required")
	}
	return &UseCase{
		sessionRepo: sessionRepo,
		llmClient:   llmClient,
	}, nil
}

// ProcessMessage processes a single inbound message and returns the response.
// It returns an error if the message is empty or the llm client fails.
// Args:
//
//	ctx: context.Context
//
//	msg: model.InboundMessage
//
// Returns:
//
//	model.OutboundMessage
//
//	error
func (uc *UseCase) ProcessMessage(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error) {
	if strings.TrimSpace(msg.Content) == "" {
		return model.OutboundMessage{}, errors.New("chat usecase: message content is empty")
	}
	// Get or create session
	session := uc.sessionRepo.GetOrCreateSession(msg.SessionKey())
	if session == nil {
		return model.OutboundMessage{}, errors.New("chat usecase: failed to get or create session")
	}

	// TODO: Update message tool context

	// TODO: Build initial messages (use get_history for LLM-formatted messages)

	// TODO: Agent loop

	// Save user message to session
	session.AddMessage(model.RoleUser, msg.Content, nil)

	// Call LLLM
	llmMessages := toLLMMessages(session.GetHistory(500))
	resp, err := uc.llmClient.Chat(ctx, llmMessages)
	if err != nil {
		return model.OutboundMessage{}, errors.New("chat usecase: failed to chat with llm")
	}

	// Save assistant message to session
	answer := strings.TrimSpace(resp.Content)
	if answer == "" {
		answer = "Sorry, I encountered an error calling the AI model."
	}
	session.AddMessage(model.RoleAssistant, answer, nil)

	// Save session
	if err := uc.sessionRepo.SaveSession(session); err != nil {
		return model.OutboundMessage{}, errors.New("chat usecase: failed to save session")
	}

	out := model.OutboundMessage{
		Channel:   msg.Channel,
		ChatID:    msg.ChatID,
		Content:   answer,
		ReplyTo:   msg.ID,
		Metadata:  map[string]any{"session_key": session.Key},
		CreatedAt: time.Now(),
	}

	return out, nil
}

// Process a message directly (for CLI usage).
// It returns an error if the message is empty or the llm client fails.
// Args:
//
//	ctx: context.Context
//
//	sessionKey: string(default: "cli:direct")
//
//	content: string(default: "")
//
// Returns:
//
//	model.OutboundMessage
//
//	error
func (uc *UseCase) ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", errors.New("chat usecase: message content is empty")
	}
	if sessionKey == "" {
		sessionKey = "cli:direct"
	}
	msg := model.InboundMessage{
		ID:       fmt.Sprintf("direct-%d", time.Now().UnixNano()),
		Channel:  model.ChannelCLI,
		SenderID: "user",
		ChatID:   "direct",
		Content:  content,
		SessionKeyOverride: func() *string {
			s := sessionKey
			return &s
		}(),
	}
	resp, err := uc.ProcessMessage(ctx, msg)
	if err != nil {
		return "", errors.New("chat usecase: failed to process message")
	}
	content = strings.TrimSpace(resp.Content)
	if content == "" {
		content = "Sorry, I encountered an error calling the AI model."
	}
	return content, nil
}

func toLLMMessages(history []*model.Message) []map[string]any {
	out := make([]map[string]any, 0, len(history))
	for _, m := range history {
		if m == nil {
			continue
		}
		item := map[string]any{
			"role":    m.Role,
			"content": m.Content,
		}
		if m.ToolCalls != nil {
			item["tool_calls"] = m.ToolCalls
		}
		if m.ToolCallID != "" {
			item["tool_call_id"] = m.ToolCallID
		}
		if m.Name != "" {
			item["name"] = m.Name
		}
		out = append(out, item)
	}
	return out
}
