package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"tinybot/internal/adapters/tool"
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
	sessionRepo   ports.SessionRepository
	llmClient     ports.LLMClient
	tools         tool.Registry
	maxIterations int
}

// NewUseCase creates a new chat use case.
// It returns an error if the session repository or llm client is nil.
func NewUseCase(sessionRepo ports.SessionRepository, llmClient ports.LLMClient, maxIterations int) (*UseCase, error) {
	if sessionRepo == nil {
		return nil, errors.New("chat usecase: session repository is required")
	}
	if llmClient == nil {
		return nil, errors.New("chat usecase: llm client is required")
	}
	return &UseCase{
		sessionRepo:   sessionRepo,
		llmClient:     llmClient,
		maxIterations: maxIterations,
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

	// Save user message to session
	session.AddMessage(model.RoleUser, msg.Content, nil)

	// Call LLLM with history message
	llmMessages := toLLMMessages(session.GetHistory(500))

	// TODO: Update message tool context

	// Agent loop
	var answer string
	for iteration := 0; iteration < uc.maxIterations; iteration++ {
		resp, err := uc.llmClient.Chat(ctx, llmMessages, uc.tools.GetDefinitions(), uc.maxIterations, 1)
		if err != nil {
			return model.OutboundMessage{}, fmt.Errorf("chat usecase llm chat: %w", err)
		}
		if resp.HasToolCalls() {
			// Append assistant's message
			toolCallMaps := make([]map[string]any, 0)
			for _, tc := range resp.ToolCalls {
				argJson, _ := json.Marshal(tc.Args)
				toolCallMaps = append(toolCallMaps, map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": string(argJson),
					},
				})
			}
			llmMessages = append(llmMessages, map[string]any{
				"role":       model.RoleAssistant,
				"content":    resp.Content,
				"tool_calls": toolCallMaps,
			})

			// Execute tool calls one by one
			for _, tc := range resp.ToolCalls {
				result, err := uc.tools.Execute(ctx, tc.Name, tc.Args)
				if err != nil {
					result = fmt.Sprintf("Error: %s", err.Error())
				}
				llmMessages = append(llmMessages, map[string]any{
					"role":         model.RoleTool,
					"tool_call_id": tc.ID,
					"name":         tc.Name,
					"content":      result,
				})
			}

			// 执行完工具得到结果后，再次调用llm
			continue
		}

		// 没有工具调用，得到答案
		answer = strings.TrimSpace(resp.Content)
		break
	}

	// Save assistant message to session

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
