package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"tinybot/internal/domain/model"
)

// EstimateTokens 粗略估算一组消息的 token 数
func EstimateTokens(messages []*model.Message) int {
	sum := 0
	for _, m := range messages {
		sum += 4
		sum += len([]rune(m.Content)) / 2
		if m.ToolCalls != nil {
			if b, err := json.Marshal(m.ToolCalls); err == nil {
				sum += len(b) / 4
			}
		}
	}
	return sum
}

// Consolidator 负责在会话历史过长时，将将旧消息压缩为摘要
//
// 工作原理：
//   - Session.LastConsolidated 记录了“已整合到第几条”的索引
//   - GetHistory 从 LastConsolidated 开始取未整合消息
//   - 当未整合消息的 token 估算超过 tokenLimit 时，触发整合
//   - 整合会保留最近 keepRecent 条消息不压缩
type Consolidator struct {
	llm        CompletionClient
	tokenLimit int // 触发整合的 token 阈值
	keepRecent int // 保留最近几条不压缩
}

// NewConsolidator 创建整合器
func NewConsolidator(llm CompletionClient, tokenLimit, keepRecent int) *Consolidator {
	if tokenLimit <= 0 {
		tokenLimit = 60000
	}
	if keepRecent <= 0 {
		keepRecent = 10
	}
	return &Consolidator{
		llm:        llm,
		tokenLimit: tokenLimit,
		keepRecent: keepRecent,
	}
}

// NeedConsolidation 判断会话的未整合消息是否超过token阈值
//
// 判断逻辑
// 1. 取出未整合消息（从 LastConsolidated 到末尾）
// 2. 估算 token 数
// 3. 同时要确保有足够的消息可以压缩（至少比 keepRecent 多）
func (c *Consolidator) NeedsConsolidation(s *model.Session) bool {
	start := s.LastConsolidated
	if start < 0 || start >= len(s.Messages) {
		start = 0
	}
	unconsolidated := s.Messages[start:]

	return EstimateTokens(unconsolidated) > c.tokenLimit && len(unconsolidated) > c.keepRecent
}

// formatForSummary 将消息列表格式化为可供 LLM 摘要的纯文本。
//
// 格式示例：
//
//	[user]: 帮我写一个排序算法
//	[assistant]: 好的，这是一个快速排序实现...
//	[tool]: {"result": "文件已保存"}
//
// 设计考量：
//   - 只保留 role 和 content，忽略 tool_call_id 等元数据
//     因为摘要关心的是"对话说了什么"，不是"技术调用链路"
//   - tool 消息只保留前 200 rune，避免长工具输出撑爆摘要 prompt
func formatForSummary(messages []*model.Message) string {
	sb := strings.Builder{}
	for _, m := range messages {
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		if m.Role == model.RoleTool && len([]rune(m.Content)) > 200 {
			fmt.Fprintf(&sb, "[%s]: %s...\n", m.Role, string([]rune(m.Content)[:200]))
		} else {
			fmt.Fprintf(&sb, "[%s]: %s\n", m.Role, m.Content)
		}
	}
	return sb.String()
}

// summarize 调用 LLM 将对话文本压缩为摘要
func (c *Consolidator) summarize(ctx context.Context, text string) (string, error) {
	prompt := `你是一个对话摘要助手。请将以下对话历史压缩为简洁的摘要。
  要求：
  - 保留所有关键事实和决策
  - 保留用户表达的偏好和要求
  - 保留未完成的任务和待办事项
  - 删除寒暄、重复内容和已解决的中间讨论
  - 用条目列表格式输出
  - 摘要长度不超过 500 字`

	messages := []map[string]any{
		{"role": "system", "content": prompt},
		{"role": "user", "content": text},
	}
	resp, err := c.llm.Chat(ctx, messages, nil, 2048, 0.2)
	if err != nil {
		return "", fmt.Errorf("summarize llm call: %w", err)
	}
	return resp.Content, nil
}

// Consolidate 执行会话整合。
//
// 流程：
// 1. 取未整合消息，按 keepRecent 拆成"待压缩"和"保留"两段
// 2. 将"待压缩"消息格式化后发给 LLM 生成摘要
// 3. 在拆分点插入一条 system 摘要消息
// 4. 更新 LastConsolidated，使 GetHistory 跳过已压缩的旧消息
//
// 整合后的消息布局：
//
//	[旧已整合消息...][本次压缩的消息...][摘要消息][保留的最近消息...]
//	                                     ↑ LastConsolidated 指向这里
func (c *Consolidator) Consolidate(ctx context.Context, session *model.Session) error {
	start := session.LastConsolidated
	if start < 0 || start >= len(session.Messages) {
		start = 0
	}
	unconsolidated := session.Messages[start:]

	// 拆分点
	splitIdx := len(unconsolidated) - c.keepRecent
	if splitIdx <= 0 {
		return nil
	}

	// 格式化待压缩消息
	toSummarize := unconsolidated[:splitIdx]
	text := formatForSummary(toSummarize)
	summary, err := c.summarize(ctx, text)
	if err != nil {
		return err
	}

	// 摘要消息
	summaryMsg := &model.Message{
		Role:    model.RoleSystem,
		Content: "[对话历史摘要]\n" + summary,
	}

	insertPos := start + splitIdx
	session.Messages = slices.Insert(session.Messages, insertPos, summaryMsg)
	session.LastConsolidated = insertPos
	return nil
}
