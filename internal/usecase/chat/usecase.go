package chat

import (
	"context"
	"encoding/json"
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
	sessionRepo   ports.SessionRepository
	llmClient     ports.LLMClient
	tools         ports.ToolRegistry
	contexts      *ContextBuilder
	maxIterations int
	maxTokens     int
}

// NewUseCase creates a new chat use case.
// It returns an error if the session repository or llm client is nil.
func NewUseCase(sessionRepo ports.SessionRepository, llmClient ports.LLMClient, tools ports.ToolRegistry, maxIterations int) (*UseCase, error) {
	if sessionRepo == nil {
		return nil, errors.New("chat usecase: session repository is required")
	}
	if llmClient == nil {
		return nil, errors.New("chat usecase: llm client is required")
	}
	if tools == nil {
		return nil, errors.New("chat usecase: tool registry is required")
	}

	return &UseCase{
		sessionRepo:   sessionRepo,
		llmClient:     llmClient,
		tools:         tools,
		contexts:      NewContextBuilder(""),
		maxIterations: maxIterations,
		maxTokens:     8192,
	}, nil
}

func (uc *UseCase) SetContextBuilder(builder *ContextBuilder) {
	if builder == nil {
		return
	}
	uc.contexts = builder
}

type traceMessage struct {
	role    string
	content string
	attrs   map[string]any
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

	// Build the LLM input with a system prompt and recent history.
	llmMessages := uc.buildLLMMessages(session.GetHistory(500), msg.Content, nil)

	// Current turn messages，save into session in the end
	trace := []traceMessage{
		{
			role:    model.RoleUser,
			content: msg.Content,
		},
	}

	// Agent loop
	var answer string
	for iteration := 0; iteration < uc.maxIterations; iteration++ {
		// 第一次迭代调用时，传入所有工具的定义
		resp, err := uc.llmClient.Chat(ctx, llmMessages, uc.tools.GetDefinitions(), uc.maxTokens, 1)
		if err != nil {
			return model.OutboundMessage{}, fmt.Errorf("chat usecase llm chat: %w", err)
		}
		// 有工具调用，需要执行工具
		if resp.HasToolCalls() {
			toolCallMaps := make([]map[string]any, 0, len(resp.ToolCalls))
			for _, tc := range resp.ToolCalls {
				argJson, _ := json.Marshal(tc.Args)
				// 工具调用的定义
				toolCallMaps = append(toolCallMaps, map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": string(argJson), // arguments is a json string
					},
				})
			}
			// 添加到历史消息中
			llmMessages = uc.contexts.AddAssistantMessage(llmMessages, resp.Content, toolCallMaps)

			trace = append(trace, traceMessage{
				role:    model.RoleAssistant,
				content: strings.TrimSpace(resp.Content),
				attrs: map[string]any{
					"tool_calls": toolCallMaps,
				},
			})

			// Execute tool calls one by one
			for _, tc := range resp.ToolCalls {
				result, err := uc.tools.Execute(ctx, tc.Name, tc.Args)
				if err != nil {
					result = fmt.Sprintf("Error: %s", err.Error())
				}
				llmMessages = uc.contexts.AddToolResult(llmMessages, tc.ID, tc.Name, result)

				trace = append(trace, traceMessage{
					role:    model.RoleTool,
					content: result,
					attrs: map[string]any{
						"tool_call_id": tc.ID,
						"name":         tc.Name,
					},
				})
			}

			// 执行完工具得到结果后，再次调用llm
			continue
		}

		// 没有工具调用，得到答案
		answer = strings.TrimSpace(resp.Content)
		break
	}

	if answer == "" {
		answer = "Sorry, I encountered an error calling the AI model."
	}

	trace = append(trace, traceMessage{
		role:    model.RoleAssistant,
		content: answer,
	})

	for _, item := range trace {
		session.AddMessage(item.role, item.content, item.attrs)
	}

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
		return "", fmt.Errorf("chat usecase: %w", err)
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

func (uc *UseCase) buildLLMMessages(history []*model.Message, currentMessage string, skillNames []string) []map[string]any {
	// TODO:skill
	if uc.contexts == nil {
		his := toLLMMessages(history)
		his = append(his, map[string]any{
			"role":    "user",
			"content": currentMessage,
		})
		return his
	}
	return uc.contexts.BuildMessages(history, currentMessage, skillNames)
}
