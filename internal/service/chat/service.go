package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"tinybot/internal/domain/model"
)

const (
	defaultMaxIterations = 20
	defaultMaxTokens     = 8192
)

// Service orchestrates one chat turn for direct calls and inbound transport messages.
//
// Responsibilities:
//   - load or create the session for the current thread
//   - build prompt messages from history and workspace context
//   - call the LLM, including iterative tool-call handling
//   - append user/tool/assistant trace items to the session and persist them
//
// Inputs: domain inbound messages or direct chat content plus an optional explicit session key.
// Outputs: a domain outbound message or plain text for direct chat helpers.
// State changes: the session grows with user, assistant, and tool messages for every processed turn.
// Side effects: calls the LLM adapter, executes tools, and writes session storage through the repository.
// Compatibility: this preserves the current Go direct-chat loop and keeps the session-key override behavior that is intentionally different from Python nanobot.
type Service struct {
	sessions      SessionRepository
	llm           CompletionClient
	tools         ToolExecutor
	prompts       PromptBuilder
	maxIterations int
	maxTokens     int
	temperature   float32
}

// NewService builds a chat service from explicit service-layer dependencies.
//
// Inputs: repository, completion client, tool executor, prompt builder, and runtime limits.
// Outputs: a ready-to-use Service or an error when a required dependency is missing.
// State changes: none during construction.
// Side effects: none during construction.
// Compatibility: the dependency set matches the existing Go MVP while making the service boundary narrower than the old ports package.
func NewService(sessions SessionRepository, llm CompletionClient, tools ToolExecutor, prompts PromptBuilder, maxIterations int, maxTokens int, temperature float32) (*Service, error) {
	if sessions == nil {
		return nil, errors.New("chat service: session repository is required")
	}
	if llm == nil {
		return nil, errors.New("chat service: completion client is required")
	}
	if tools == nil {
		return nil, errors.New("chat service: tool executor is required")
	}
	if prompts == nil {
		return nil, errors.New("chat service: prompt builder is required")
	}
	if maxIterations <= 0 {
		maxIterations = defaultMaxIterations
	}
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}

	return &Service{
		sessions:      sessions,
		llm:           llm,
		tools:         tools,
		prompts:       prompts,
		maxIterations: maxIterations,
		maxTokens:     maxTokens,
		temperature:   temperature,
	}, nil
}

type traceMessage struct {
	role    string
	content string
	attrs   map[string]any
}

// ProcessMessage handles one inbound message end-to-end.
//
// Inputs: a populated InboundMessage that already identifies the transport channel and chat thread.
// Outputs: the assistant's outbound reply plus metadata that includes the resolved session key.
// State changes: appends the full turn trace to the session and persists it.
// Side effects: may call the model, execute tools, and write to session storage.
// Compatibility: the control flow matches nanobot's `process_direct` / agent loop sequence: session -> prompt -> LLM -> tools -> save session.
func (s *Service) ProcessMessage(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error) {
	if strings.TrimSpace(msg.Content) == "" {
		return model.OutboundMessage{}, errors.New("chat service: message content is empty")
	}

	session, err := s.sessions.GetOrCreateSession(ctx, msg.SessionKey())
	if err != nil {
		return model.OutboundMessage{}, fmt.Errorf("chat service get or create session: %w", err)
	}
	if session == nil {
		return model.OutboundMessage{}, errors.New("chat service: failed to get or create session")
	}

	llmMessages := s.prompts.BuildMessages(session.GetHistory(500), msg.Content, nil)
	trace := []traceMessage{{
		role:    model.RoleUser,
		content: msg.Content,
	}}

	var answer string
	for iteration := 0; iteration < s.maxIterations; iteration++ {
		resp, err := s.llm.Chat(ctx, llmMessages, s.tools.GetDefinitions(), s.maxTokens, s.temperature)
		if err != nil {
			return model.OutboundMessage{}, fmt.Errorf("chat service llm chat: %w", err)
		}

		if resp.HasToolCalls() {
			toolCallMaps := make([]map[string]any, 0, len(resp.ToolCalls))
			for _, tc := range resp.ToolCalls {
				argJSON, _ := json.Marshal(tc.Args)
				toolCallMaps = append(toolCallMaps, map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Name,
						"arguments": string(argJSON),
					},
				})
			}

			llmMessages = s.prompts.AddAssistantMessage(llmMessages, resp.Content, toolCallMaps)
			trace = append(trace, traceMessage{
				role:    model.RoleAssistant,
				content: strings.TrimSpace(resp.Content),
				attrs: map[string]any{
					"tool_calls": toolCallMaps,
				},
			})

			for _, tc := range resp.ToolCalls {
				result, err := s.tools.Execute(ctx, tc.Name, tc.Args)
				if err != nil {
					result = fmt.Sprintf("Error: %s", err.Error())
				}

				llmMessages = s.prompts.AddToolResult(llmMessages, tc.ID, tc.Name, result)
				trace = append(trace, traceMessage{
					role:    model.RoleTool,
					content: result,
					attrs: map[string]any{
						"tool_call_id": tc.ID,
						"name":         tc.Name,
					},
				})
			}

			continue
		}

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

	if err := s.sessions.SaveSession(ctx, session); err != nil {
		return model.OutboundMessage{}, fmt.Errorf("chat service save session: %w", err)
	}

	return model.OutboundMessage{
		Channel:   msg.Channel,
		ChatID:    msg.ChatID,
		Content:   answer,
		ReplyTo:   msg.ID,
		Metadata:  map[string]any{"session_key": session.Key},
		CreatedAt: time.Now(),
	}, nil
}

// ProcessDirect handles the direct CLI-style chat path without a transport loop.
//
// Inputs: a context, an explicit session key, and raw user text.
// Outputs: the assistant reply as plain text.
// State changes: persists the turn into the session identified by the explicit session key.
// Side effects: same as ProcessMessage, because this is a thin wrapper around the main orchestration path.
// Compatibility: unlike Python nanobot, the explicit session key is preserved intentionally in the Go rewrite so direct CLI calls can control thread boundaries.
func (s *Service) ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", errors.New("chat service: message content is empty")
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

	resp, err := s.ProcessMessage(ctx, msg)
	if err != nil {
		return "", fmt.Errorf("chat service: %w", err)
	}

	content = strings.TrimSpace(resp.Content)
	if content == "" {
		content = "Sorry, I encountered an error calling the AI model."
	}
	return content, nil
}

func toLLMMessages(history []*model.Message) []map[string]any {
	out := make([]map[string]any, 0, len(history))
	for _, msg := range history {
		if msg == nil {
			continue
		}

		item := map[string]any{
			"role":    msg.Role,
			"content": msg.Content,
		}
		if msg.ToolCalls != nil {
			item["tool_calls"] = msg.ToolCalls
		}
		if msg.ToolCallID != "" {
			item["tool_call_id"] = msg.ToolCallID
		}
		if msg.Name != "" {
			item["name"] = msg.Name
		}
		out = append(out, item)
	}
	return out
}
