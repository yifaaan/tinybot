package model

import (
	"fmt"
	"time"
)

/*

 // 进来的消息 — 系统需要知道很多信息来处理它
  InboundMessage{
      ID:         "msg-001",
      Channel:    ChannelTelegram,
      SenderID:   "user-12345",           // 谁发的？要做权限校验
      ChatID:     "chat-67890",
      SessionKey: "telegram:chat-67890",  // 路由到哪个会话？要加载历史
      Content:    "这是什么花？",
      MediaURLs:  []string{"https://..."},// 用户发了一张图
      CreatedAt:  time.Now(),
  }

  // 出去的消息 — 只需要知道发到哪里、回复什么
  OutboundMessage{
      ID:        "msg-002",
      Channel:   ChannelTelegram,
      ChatID:    "chat-67890",           // 发到哪个对话
      Content:   "这是一朵玫瑰花。",
      ReplyTo:   "msg-001",             // 回复哪条消息
      CreatedAt: time.Now(),
  }

*/

// Channel represents a message source/destination.
type Channel string

const (
	ChannelCLI       Channel = "cli"
	ChannelTelegram  Channel = "telegram"
	ChannelWhatsApp  Channel = "whatsapp"
	ChannelCron      Channel = "cron"
	ChannelHeartbeat Channel = "heartbeat"
)

// Message received from a chat channel.
type InboundMessage struct {
	ID                 string
	Channel            Channel
	SenderID           string // User identifier
	ChatID             string // Chat/channel identifier
	Content            string // Message text
	MediaURLs          []string
	SelectedSkills     []string // Optional per-turn skill activation
	Metadata           map[string]any
	CreatedAt          time.Time
	SessionKeyOverride *string // Optional override for thread-scoped sessions

	// StreamWriter 可选的流式输出写入器。
	// 由 Channel 在创建消息时注入，GatewayLoop 检测到非 nil 时启用流式输出。
	StreamWriter StreamWriter
}

func (m InboundMessage) SessionKey() string {
	if m.SessionKeyOverride != nil {
		return *m.SessionKeyOverride
	}
	return fmt.Sprintf("%s:%s", m.Channel, m.ChatID)
}

// Message to sendt to a chat channel.
type OutboundMessage struct {
	ID        string
	Channel   Channel
	ChatID    string
	Content   string
	ReplyTo   string
	Metadata  map[string]any
	CreatedAt time.Time

	// Streamed 标记此消息是否已经流式输出过内容。
	// 为 true 时，Channel.Send() 只需打印换行和提示符，不需要再打印 Content。
	Streamed bool
}

// StreamWriter 定义流式输出写入能力
//
// - ConsoleChannel 可以直接实现它
// - TelegramChannel 可能需要不同的实现（分块发送、Markdown 解析等）
type StreamWriter interface {
	WriteDelta(delta string) error
}
