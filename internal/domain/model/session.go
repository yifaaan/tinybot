package model

import (
	"time"
)

// Role constants for chat messages.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
	RoleSystem    = "system"
)

type Message struct {
	Role       string    `json:"role"` // user, assistant, tool, system
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	ToolCalls  any       `json:"tool_calls,omitempty"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
	Name       string    `json:"name,omitempty"`
}

type Session struct {
	Key              string         `json:"key"` // channel:ChatID
	Messages         []*Message     `json:"-"`   // stored in JSONL entries, not in metadata line
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	Metadata         map[string]any `json:"metadata"`
	LastConsolidated int            `json:"last_consolidated"` // Number of messages already consolidated
}

func NewSession(key string) *Session {
	now := time.Now()
	return &Session{
		Key:       key,
		Messages:  make([]*Message, 0),
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  make(map[string]any),
	}
}

// AddMessage adds a new message and updates the timestamp.
func (s *Session) AddMessage(role, content string, attrs map[string]any) {
	msg := &Message{
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}

	if attrs != nil {
		if v, ok := attrs["tool_calls"]; ok {
			msg.ToolCalls = v
		}
		if v, ok := attrs["tool_call_id"]; ok {
			if id, ok := v.(string); ok {
				msg.ToolCallID = id
			}
		}
		if v, ok := attrs["name"]; ok {
			if name, ok := v.(string); ok {
				msg.Name = name
			}
		}
	}

	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// GetHistory returns unconsolidated history capped at n (default 500),
// and trims leading non-user messages to avoid orphan tool chains.
func (s *Session) GetHistory(n int) []*Message {
	if n <= 0 {
		n = 500
	}

	start := s.LastConsolidated
	if start < 0 || start >= len(s.Messages) {
		start = 0
	}
	unconsolidated := s.Messages[start:]
	if len(unconsolidated) > n {
		unconsolidated = unconsolidated[len(unconsolidated)-n:]
	}

	sliced := unconsolidated
	// Drop leading non-user messages to avoid orphaned tool_result blocks
	// Tool call 链必须是完整的：user -> assistant (带 tool_calls) → tool (返回 tool_result) → assistant (处理结果)
	// 但保留 RoleSystem 消息（如 consolidation 摘要），它们是有效的上下文
	for i, m := range sliced {
		if m.Role == RoleUser || m.Role == RoleSystem {
			sliced = sliced[i:]
			break
		}
	}
	return sliced
}
