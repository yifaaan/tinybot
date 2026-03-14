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
// 区别只在“谁触发了一次 turn”，而不是“turn 内部怎么做”。
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

// messageContextSetter 是一个局部窄接口。
// chat service 不需要知道具体的 Registry 类型，
// 只需要知道“如果工具集合支持更新 message 上下文，那就在每轮开始时调用一下”
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
// - 这和“完成一个 turn 后再整体保存”的思路更一致
type traceMessage struct {
	role    string
	content string
	attrs   map[string]any
}

// ProcessMessage 处理一条标准化后的输入消息，并返回最终回复。
//
// 参数：
// - ctx: 请求级上下文
// - msg: 已经携带 channel/chat/thread 信息的输入消息
//
// 返回：
// - model.OutboundMessage: 可继续交给 transport 层投递的回复
// - error: 整轮处理失败时返回错误
//
// 为什么这层要同时处理 direct chat 和 bus 驱动消息：
// - 它们的核心行为完全相同，区别只在“消息从哪里来、结果发到哪里去”
// - 如果分别维护两套 turn 逻辑，后续几乎一定会发生行为漂移
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

	// 压缩旧消息
	if s.consolidator != nil && s.consolidator.NeedsConsolidation(session) {
		if err := s.consolidator.Consolidate(ctx, session); err != nil {
			// TODO:log
		}
	}
	// 每轮进入tool loop前，帮inbound的channel/chatid 传给message tool
	if setter, ok := s.tools.(messageContextSetter); ok {
		setter.SetMessageContext(msg.Channel, msg.ChatID)
	}
	// Explicit per-turn skill selection should flow through the request boundary.
	llmMessages := s.prompts.BuildMessages(session.GetHistory(500), msg.Content, msg.SelectedSkills)
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
				// 这里把内部 ToolCall 重新编码成 provider 常见的 tool-call message 形状，
				// 目的是让下一轮模型调用能够“看见自己刚才请求了什么工具”。
				//
				// 如果缺少这一步，模型下一轮只会收到 tool result，却不知道这个结果是怎么来的。
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

			// 先把 assistant 的 tool-call 痕迹写回消息列表，再执行真实工具。
			// 这样 transcript 的顺序和真实推理过程保持一致，也更容易调试。
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
					// 这里故意不因为单个工具失败而直接终止整轮 turn。
					// 原因是模型经常可以根据错误文本决定是否重试、换参数，
					// 或者直接解释失败；这比让整轮对话直接崩掉更有恢复力。
					result = fmt.Sprintf("Error: %s", err.Error())
				}

				// 把工具结果继续追加回 transcript，供下一轮模型消费。
				// assistant 的 tool-call trace 与 tool result trace 必须成对出现，tool loop 才完整。
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

		// 一旦模型不再请求工具，就把当前内容视为本轮最终回答。
		// TrimSpace 是为了避免 provider 返回纯空白时被误判成有效输出。
		answer = strings.TrimSpace(resp.Content)
		break
	}

	// 如果多轮之后依然没有产出最终回答，先给一个稳定兜底文案，
	// 避免 direct chat 最后返回空字符串。
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
