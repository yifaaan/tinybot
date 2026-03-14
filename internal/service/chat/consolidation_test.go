package chat

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"tinybot/internal/domain/model"
)

func TestEstimateTokens_EmptySlice(t *testing.T) {
	got := EstimateTokens(nil)
	if got != 0 {
		t.Errorf("EstimateTokens(nil) = %d, want 0", got)
	}
}

func TestEstimateTokens_SingleShortMessage(t *testing.T) {
	msgs := []*model.Message{
		{Role: "user", Content: "hello"},
	}
	got := EstimateTokens(msgs)
	if got < 3 || got > 15 {
		t.Errorf("EstimateTokens short msg = %d, want 3~15", got)
	}
}

func TestEstimateTokens_ChineseContent(t *testing.T) {
	msgs := []*model.Message{
		{Role: "user", Content: "你好世界，这是一段中文测试消息"},
	}
	got := EstimateTokens(msgs)
	// 14 runes → 14/2 = 7, + 4 = 11
	if got < 5 || got > 25 {
		t.Errorf("EstimateTokens chinese = %d, want 5~25", got)
	}
}

func TestEstimateTokens_MultipleMessages(t *testing.T) {
	msgs := []*model.Message{
		{Role: "user", Content: "first message"},
		{Role: "assistant", Content: "second message with more content"},
		{Role: "user", Content: "third"},
	}
	got := EstimateTokens(msgs)
	// 3 条消息，每条 4 开销 = 12，加上内容估算，总数应在 20~60 之间
	if got < 15 || got > 80 {
		t.Errorf("EstimateTokens multi = %d, want 15~80", got)
	}
}

func TestNewConsolidator_Defaults(t *testing.T) {
	c := NewConsolidator(nil, 0, 0)
	if c.tokenLimit != 60000 {
		t.Errorf("tokenLimit = %d, want 60000", c.tokenLimit)
	}
	if c.keepRecent != 10 {
		t.Errorf("keepRecent = %d, want 10", c.keepRecent)
	}
}

func TestNeedsConsolidation_BelowThreshold(t *testing.T) {
	c := NewConsolidator(nil, 1000, 2)
	s := &model.Session{
		Messages: []*model.Message{
			{Role: "user", Content: "short"},
			{Role: "assistant", Content: "reply"},
		},
	}
	if c.NeedsConsolidation(s) {
		t.Error("should not need consolidation for short session")
	}
}

func TestNeedsConsolidation_AboveThreshold(t *testing.T) {
	// 构造一个超过阈值的会话
	c := NewConsolidator(nil, 10, 2)
	msgs := make([]*model.Message, 20)
	for i := range msgs {
		msgs[i] = &model.Message{Role: "user", Content: "this is a long enough message to push tokens over the limit"}
	}
	s := &model.Session{Messages: msgs}
	if !c.NeedsConsolidation(s) {
		t.Error("should need consolidation for long session")
	}
}

func TestNeedsConsolidation_NotEnoughToCompress(t *testing.T) {
	// 消息总数 <= keepRecent，没东西可压缩
	c := NewConsolidator(nil, 10, 20)
	msgs := make([]*model.Message, 5)
	for i := range msgs {
		msgs[i] = &model.Message{Role: "user", Content: "some content here"}
	}
	s := &model.Session{Messages: msgs}
	if c.NeedsConsolidation(s) {
		t.Error("should not need consolidation when not enough messages to compress")
	}
}

func TestNeedsConsolidation_RespectsLastConsolidated(t *testing.T) {
	c := NewConsolidator(nil, 10, 2)
	msgs := make([]*model.Message, 20)
	for i := range msgs {
		msgs[i] = &model.Message{Role: "user", Content: "long message content here"}
	}
	s := &model.Session{
		Messages:         msgs,
		LastConsolidated: 18,
	}
	if c.NeedsConsolidation(s) {
		t.Error("should not need consolidation when unconsolidated <= keepRecent")
	}
}

func TestFormatForSummary_Basic(t *testing.T) {
	msgs := []*model.Message{
		{Role: "user", Content: "你好"},
		{Role: "assistant", Content: "你好！有什么可以帮你的？"},
	}
	got := formatForSummary(msgs)
	if !strings.Contains(got, "[user]: 你好") {
		t.Errorf("missing user message, got:\n%s", got)
	}
	if !strings.Contains(got, "[assistant]: 你好！有什么可以帮你的？") {
		t.Errorf("missing assistant message, got:\n%s", got)
	}
}

func TestFormatForSummary_ToolTruncation(t *testing.T) {
	longContent := strings.Repeat("あ", 300) // 300 rune
	msgs := []*model.Message{
		{Role: "tool", Content: longContent},
	}
	got := formatForSummary(msgs)
	// 应被截断到 200 rune + "..."
	if strings.Contains(got, strings.Repeat("あ", 201)) {
		t.Error("tool content should be truncated to 200 runes")
	}
	if !strings.Contains(got, "...") {
		t.Error("truncated content should end with ...")
	}
}

func TestFormatForSummary_SkipsEmpty(t *testing.T) {
	msgs := []*model.Message{
		{Role: "user", Content: ""},
		{Role: "assistant", Content: "real reply"},
	}
	got := formatForSummary(msgs)
	if strings.Contains(got, "[user]") {
		t.Error("should skip empty content messages")
	}
	if !strings.Contains(got, "[assistant]: real reply") {
		t.Errorf("should contain non-empty message, got:\n%s", got)
	}
}

// mockLLM 实现 CompletionClient 接口，用于测试
type mockLLM struct {
	response model.LLMResponse
	err      error
}

func (m *mockLLM) Chat(_ context.Context, _ []map[string]any, _ []map[string]any, _ int, _ float32) (model.LLMResponse, error) {
	return m.response, m.err
}

func TestConsolidate_Basic(t *testing.T) {
	llm := &mockLLM{
		response: model.LLMResponse{Content: "- 用户讨论了排序算法\n- 决定使用快速排序"},
	}
	c := NewConsolidator(llm, 10, 2) // 低阈值，keepRecent=2

	msgs := make([]*model.Message, 10)
	for i := range msgs {
		msgs[i] = &model.Message{
			Role:    "user",
			Content: fmt.Sprintf("message number %d with enough content", i),
		}
	}
	session := &model.Session{Messages: msgs, LastConsolidated: 0}

	err := c.Consolidate(context.Background(), session)
	if err != nil {
		t.Fatalf("Consolidate error: %v", err)
	}

	// 验证 LastConsolidated 已更新
	if session.LastConsolidated == 0 {
		t.Error("LastConsolidated should have been updated")
	}

	// 验证摘要消息存在
	summaryMsg := session.Messages[session.LastConsolidated]
	if summaryMsg.Role != model.RoleSystem {
		t.Errorf("summary msg role = %s, want system", summaryMsg.Role)
	}
	if !strings.Contains(summaryMsg.Content, "[对话历史摘要]") {
		t.Errorf("summary msg should contain header, got: %s", summaryMsg.Content)
	}

	// 验证消息总数增加了 1（摘要消息被插入）
	// 原本 10 条，插入摘要后应为 11 条
	if len(session.Messages) != 11 {
		t.Errorf("expected 11 messages after consolidation, got %d", len(session.Messages))
	}
	// 验证摘要消息后面是保留的最近消息
	nextMsg := session.Messages[session.LastConsolidated+1]
	if !strings.Contains(nextMsg.Content, "message number 8") {
		t.Errorf("message after summary should be msg 8, got: %s", nextMsg.Content)
	}
}

func TestConsolidate_NothingToCompress(t *testing.T) {
	c := NewConsolidator(nil, 10, 20) // keepRecent=20 大于消息数
	msgs := make([]*model.Message, 5)
	for i := range msgs {
		msgs[i] = &model.Message{Role: "user", Content: "short"}
	}
	session := &model.Session{Messages: msgs}

	err := c.Consolidate(context.Background(), session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// LastConsolidated 不应变化
	if session.LastConsolidated != 0 {
		t.Error("LastConsolidated should remain 0")
	}
}

func TestConsolidate_LLMError(t *testing.T) {
	llm := &mockLLM{err: fmt.Errorf("api timeout")}
	c := NewConsolidator(llm, 10, 2)

	msgs := make([]*model.Message, 10)
	for i := range msgs {
		msgs[i] = &model.Message{Role: "user", Content: "some content here"}
	}
	session := &model.Session{Messages: msgs}

	err := c.Consolidate(context.Background(), session)
	if err == nil {
		t.Fatal("expected error from LLM failure")
	}
	if !strings.Contains(err.Error(), "api timeout") {
		t.Errorf("error should contain cause, got: %v", err)
	}
}
