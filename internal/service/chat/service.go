// Package chat 提供单次 agent turn 的核心编排逻辑。
//
// 这一层真正负责的事情只有一条主链路：
// - 载入 session
// - 组 prompt
// - 调模型
// - 跑 tool loop
// - 保存 trace
//
// direct chat、gateway、cron、heartbeat 最终都应复用这条业务路径，
// 区别只在"谁触发了一次 turn"，而不是"turn 内部怎么做"。
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"tinybot/internal/domain/model"
	"tinybot/internal/utils/logger"
)

const (
	defaultMaxIterations = 20
	defaultMaxTokens     = 8192
)

// messageContextSetter 是一个局部窄接口。
// chat service 不需要知道具体的 Registry 类型，
// 只需要知道"如果工具集合支持更新 message 上下文，那就在每轮开始时调用一下"
type messageContextSetter interface {
	SetMessageContext(channel model.Channel, chatID string)
}

// Service 负责完成一次完整的 agent 对话回合。
//
// 它的职责是稳定地执行下面这条主链路：
// 1. 载入或创建 session
// 2. 组装 prompt
// 3. 调用模型
// 4. 处理工具循环
// 5. 保存本轮 trace
type Service struct {
	sessions      SessionRepository
	llm           CompletionClient
	tools         ToolExecutor
	prompts       PromptBuilder
	maxIterations int
	maxTokens     int
	temperature   float32
	consolidator  *Consolidator
}

// NewService 构造一个可执行单轮对话的 chat service。
//
// 参数：
// - sessions: session 持久化边界
// - llm: 模型调用边界
// - tools: 工具定义与执行边界
// - prompts: prompt 装配边界
// - maxIterations: 最大工具循环次数，<= 0 时使用默认值
// - maxTokens: 单次模型输出 token 上限，<= 0 时使用默认值
// - temperature: 模型采样温度
//
// 返回：
// - *Service: 初始化好的 service
// - error: 依赖缺失时返回错误
func NewService(sessions SessionRepository, llm CompletionClient, tools ToolExecutor, prompts PromptBuilder, maxIterations int, maxTokens int, temperature float32, consolidator *Consolidator) (*Service, error) {
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
		consolidator:  consolidator,
	}, nil
}

// traceMessage 是本轮对话的临时痕迹结构。
//
// 为什么单独有一层 trace，而不是边执行边立刻写入 session：
// - tool loop 可能要跑多轮，先在内存里收集整轮痕迹更容易保证顺序正确
// - 如果中途 provider 调用失败，就不会留下半截、难解释的 session
// - 这和"完成一个 turn 后再整体保存"的思路更一致
type traceMessage struct {
	role    string
	content string
	attrs   map[string]any
}

// llmCallFn 封装了"调用一次 LLM"这个唯一的行为差异点。
//
// 为什么不用接口而用函数类型：
// - 差异只有一个动作（调用模型），不需要方法集
// - 函数闭包可以直接捕获 onDelta 等外部状态，比接口更轻量
// - 这是 Go 里最地道的"策略模式"实现方式
type llmCallFn func(ctx context.Context, messages []map[string]any, tools []map[string]any) (model.LLMResponse, error)

// processTurn 是 ProcessMessage 和 ProcessMessageStream 的共享内核。
//
// 把 session 加载 → consolidation → prompt 构建 → tool loop → trace 保存
// 这条完整链路集中在一个方法里，避免两套公开方法各维护一份 tool loop 导致行为漂移。
//
// "如何调用模型"是两条路径唯一的差异，通过 callLLM 参数注入：
// - 非流式：直接包装 s.llm.Chat(...)
// - 流式：消费 ChatStream channel、推送 delta、拼出完整 LLMResponse
func (s *Service) processTurn(ctx context.Context, msg model.InboundMessage, callLLM llmCallFn) (model.OutboundMessage, error) {
	logger.Debug("process turn start", "session", msg.SessionKey(), "content_len", len(msg.Content))

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

	if s.consolidator != nil && s.consolidator.NeedsConsolidation(session) {
		if err := s.consolidator.Consolidate(ctx, session); err != nil {
			logger.Warn("session consolidation failed", "session", session.Key, "error", err)
		}
	}
	if setter, ok := s.tools.(messageContextSetter); ok {
		setter.SetMessageContext(msg.Channel, msg.ChatID)
	}

	llmMessages := s.prompts.BuildMessages(session.GetHistory(500), msg.Content, msg.SelectedSkills)
	trace := []traceMessage{{
		role:    model.RoleUser,
		content: msg.Content,
	}}

	var answer string
	var lastThinking string
	for iteration := 0; iteration < s.maxIterations; iteration++ {
		logger.Debug("llm call iteration", "session", session.Key, "iteration", iteration+1)

		// callLLM 的具体实现由调用方决定：
		// - ProcessMessage 传入的闭包直接调 s.llm.Chat()
		// - ProcessMessageStream 传入的闭包消费 ChatStream channel 并推送 delta
		resp, err := callLLM(ctx, llmMessages, s.tools.GetDefinitions())
		if err != nil {
			logger.Error("llm call failed", "session", session.Key, "error", err)
			return model.OutboundMessage{}, fmt.Errorf("chat service llm call: %w", err)
		}

		if resp.HasToolCalls() {
			logger.Debug("tool calls received", "session", session.Key, "count", len(resp.ToolCalls))

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

			traceAttrs := map[string]any{"tool_calls": toolCallMaps}
			if resp.Thinking != "" {
				traceAttrs["thinking"] = resp.Thinking
			}
			trace = append(trace, traceMessage{
				role:    model.RoleAssistant,
				content: strings.TrimSpace(resp.Content),
				attrs:   traceAttrs,
			})

			for _, tc := range resp.ToolCalls {
				logger.Debug("executing tool", "session", session.Key, "tool", tc.Name)

				result, err := s.tools.Execute(ctx, tc.Name, tc.Args)
				if err != nil {
					logger.Warn("tool execution failed", "session", session.Key, "tool", tc.Name, "error", err)
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
		lastThinking = strings.TrimSpace(resp.Thinking)
		logger.Debug("got final answer", "session", session.Key, "answer_len", len(answer), "thinking_len", len(lastThinking))
		break
	}

	if answer == "" {
		logger.Warn("empty answer after all iterations", "session", session.Key)
		answer = "Sorry, I encountered an error calling the AI model."
	}

	// 最终回答的 trace：如果模型有 thinking 输出，一并记录到 attrs
	// 但 thinking 不会进入 llmMessages，不会作为历史上下文回传给模型
	finalTrace := traceMessage{
		role:    model.RoleAssistant,
		content: answer,
	}
	if lastThinking != "" {
		finalTrace.attrs = map[string]any{"thinking": lastThinking}
	}
	trace = append(trace, finalTrace)

	for _, item := range trace {
		session.AddMessage(item.role, item.content, item.attrs)
	}

	if err := s.sessions.SaveSession(ctx, session); err != nil {
		logger.Error("save session failed", "session", session.Key, "error", err)
		return model.OutboundMessage{}, fmt.Errorf("chat service save session: %w", err)
	}

	logger.Info("turn completed", "session", session.Key, "answer_len", len(answer))

	return model.OutboundMessage{
		Channel:   msg.Channel,
		ChatID:    msg.ChatID,
		Content:   answer,
		ReplyTo:   msg.ID,
		Metadata:  map[string]any{"session_key": session.Key},
		CreatedAt: time.Now(),
	}, nil
}

// ProcessMessage 处理一条标准化后的输入消息，返回最终回复。
//
// 委托给 processTurn，传入非流式的 llmCallFn 闭包。
func (s *Service) ProcessMessage(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error) {
	return s.processTurn(ctx, msg, func(ctx context.Context, messages []map[string]any, tools []map[string]any) (model.LLMResponse, error) {
		return s.llm.Chat(ctx, messages, tools, s.maxTokens, s.temperature)
	})
}

// ProcessDirect 处理 direct CLI 风格的单轮输入。
//
// 参数：
// - ctx: 请求级上下文
// - sessionKey: 显式 session key；为空时回退到默认 direct 会话
// - content: 用户输入文本
//
// 返回：
// - string: assistant 最终回复
// - error: 处理失败时返回错误
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

// ProcessMessageStream 与 ProcessMessage 功能相同，但通过回调逐步推送增量内容。
//
// 回调说明：
// - onDelta: 正文增量（每个文本片段到达时调用）
// - onThinking: 推理过程增量（模型 thinking 阶段的每个片段）；可为 nil 表示不关心
//
// 如果 llm 不支持 StreamingCompletionClient，自动回退到非流式 ProcessMessage。
// 完整回复仍写入 session trace，保证持久化不受影响。
func (s *Service) ProcessMessageStream(ctx context.Context, msg model.InboundMessage, onDelta func(delta string), onThinking func(delta string)) (model.OutboundMessage, error) {
	streamer, canStream := s.llm.(StreamingCompletionClient)
	if !canStream {
		return s.ProcessMessage(ctx, msg)
	}

	return s.processTurn(ctx, msg, func(ctx context.Context, messages []map[string]any, tools []map[string]any) (model.LLMResponse, error) {
		var resp *model.LLMResponse
		for event := range streamer.ChatStream(ctx, messages, tools, s.maxTokens, s.temperature) {
			switch event.Kind {
			case model.StreamEventThinking:
				if onThinking != nil {
					onThinking(event.Delta)
				}
			case model.StreamEventDelta:
				if onDelta != nil {
					onDelta(event.Delta)
				}
			case model.StreamEventDone:
				resp = event.Response
			case model.StreamEventError:
				return model.LLMResponse{}, event.Err
			}
		}
		if resp == nil {
			return model.LLMResponse{}, errors.New("stream ended without response")
		}
		return *resp, nil
	})
}

// toLLMMessages 把持久化层的 Message 结构转换成 provider 可消费的消息形状。
//
// 为什么需要这一层转换：
// - session 中保存的是领域模型，字段命名和结构更偏向业务可读性
// - provider 需要的是另一种更扁平的消息形状
// - 把转换逻辑集中在这里，可以避免 provider-specific 细节散落在 service 和 builder 中
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
