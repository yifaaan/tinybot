package channel

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"tinybot/internal/domain/model"
	"tinybot/internal/transport"
	"tinybot/internal/utils/logger"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const defaultEditInterval = 500 * time.Millisecond

// TelegramChannel implements the Channel interface for Telegram Bot API.
// 使用 long polling 模式接收消息，通过 SendMessage API 发送回复
// 支持流式输出：通过 TelegramStreamWriter 先发占位消息，再用 editMessageText 逐步更新。
type TelegramChannel struct {
	bus   transport.MessageBus
	token string
	bot   *tgbotapi.BotAPI

	// activeWriters 按入站 msgID 维护流式输出写入器
	// 当 Send 收到 Streamed == true 时，通过 ReplyTo 找到对应的写入器并更新内容
	activeWriters map[string]*TelegramStreamWriter
	mu            sync.Mutex
}

// NewTelegramChannel creates a new Telegram channel.
func NewTelegramChannel(bus transport.MessageBus, token string) (*TelegramChannel, error) {
	if bus == nil {
		return nil, fmt.Errorf("telegram: message bus is nil")
	}
	if token == "" {
		return nil, fmt.Errorf("telegram: token is empty")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("telegram: create bot: %w", err)
	}

	return &TelegramChannel{
		bus:           bus,
		token:         token,
		bot:           bot,
		activeWriters: make(map[string]*TelegramStreamWriter),
	}, nil
}

// Name returns the channel identifier.
func (c *TelegramChannel) Name() model.Channel {
	return model.ChannelTelegram
}

// Start begins long polling for updates and publishes inbound messages.
// 使用 tgbotapi.UpdateConfig 配置 long polling 参数
// 循环获取更新，将每个消息转换为 InboundMessage 并发布到 bus
func (c *TelegramChannel) Start(ctx context.Context) error {
	logger.Info("telegram channel started")

	ucfg := tgbotapi.NewUpdate(0)
	ucfg.Timeout = 60

	updates := c.bot.GetUpdatesChan(ucfg)

	for {
		select {
		case <-ctx.Done():
			logger.Info("telegram channel stopped")
			return nil
		case update, ok := <-updates:
			if !ok {
				continue
			}
			logger.Debug("received update", "message_id", update.Message.MessageID)
			c.handleUpdate(ctx, update)
		}
	}
}

// handleUpdate converts a Telegram update to an InboundMessage and publishes it.
func (c *TelegramChannel) handleUpdate(ctx context.Context, update tgbotapi.Update) error {
	// 忽略非消息
	if update.Message == nil {
		return nil
	}

	chatID := update.Message.Chat.ID
	msgID := fmt.Sprintf("tg-%d", update.Message.MessageID)

	// 每条入站消息创建独立的 StreamWriter
	writer := newTelegramStreamWriter(c.bot, chatID)

	c.mu.Lock()
	c.activeWriters[msgID] = writer
	c.mu.Unlock()

	msg := model.InboundMessage{
		ID:           msgID,
		Channel:      model.ChannelTelegram,
		SenderID:     strconv.FormatInt(update.Message.From.ID, 10),
		ChatID:       strconv.FormatInt(update.Message.Chat.ID, 10),
		Content:      update.Message.Text,
		CreatedAt:    time.Unix(int64(update.Message.Date), 0),
		StreamWriter: writer,
	}

	return c.bus.PublishInbound(ctx, msg)
}

func (c *TelegramChannel) Send(ctx context.Context, out model.OutboundMessage) error {
	if out.Streamed {
		c.mu.Lock()
		writer, ok := c.activeWriters[out.ReplyTo]
		delete(c.activeWriters, out.ReplyTo)
		c.mu.Unlock()

		if ok {
			return writer.finalFlush()
		}
		return nil
	}
	logger.Debug("sending message", "chat_id", out.ChatID, "content_len", len(out.Content))

	chatID, err := strconv.ParseInt(out.ChatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: parse chat_id: %w", err)
	}

	msg := tgbotapi.NewMessage(chatID, out.Content)

	_, err = c.bot.Send(msg)
	if err != nil {
		return fmt.Errorf("telegram: send message: %w", err)
	}
	return nil
}

type TelegramStreamWriter struct {
	bot          *tgbotapi.BotAPI
	chatID       int64
	messageID    int // 0 = 未发送的初始消息
	content      strings.Builder
	dirty        bool // 自上次 edit 以来是否有新增内容
	lastEditAt   time.Time
	editInterval time.Duration
	mu           sync.Mutex
}

func newTelegramStreamWriter(bot *tgbotapi.BotAPI, chatID int64) *TelegramStreamWriter {
	return &TelegramStreamWriter{
		bot:          bot,
		chatID:       chatID,
		editInterval: defaultEditInterval,
	}
}

// WriteDelta 累积增量文本，满足 debounce 条件时发送/更新 Telegram 消息。
func (w *TelegramStreamWriter) WriteDelta(delta string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.content.WriteString(delta)
	w.dirty = true

	now := time.Now()

	if w.messageID == 0 {
		return w.sendInitial(now)
	}

	// 距离上次 edit 超过时间间隔：更新消息
	if now.Sub(w.lastEditAt) >= w.editInterval {
		return w.editCurrent(now)
	}

	// 正常情况：不更新消息,累计内容 或 finalFlush
	return nil
}

func (w *TelegramStreamWriter) Flush() error {
	return nil
}

// finalFlush 无视 debounce 间隔，强制同步累积内容到 Telegram。
// 由 TelegramChannel.Send() 在流式输出结束后调用。
func (w *TelegramStreamWriter) finalFlush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.dirty || w.content.Len() == 0 {
		return nil
	}
	if w.messageID == 0 {
		return w.sendInitial(time.Now())
	}

	return w.editCurrent(time.Now())
}

// sendInitial 发送初始占位消息，保存 messageID 并设置 dirty=false。
func (w *TelegramStreamWriter) sendInitial(now time.Time) error {
	msg := tgbotapi.NewMessage(w.chatID, w.content.String())
	sent, err := w.bot.Send(msg)
	if err != nil {
		return fmt.Errorf("telegram: send initial message: %w", err)
	}
	w.messageID = sent.MessageID
	w.dirty = false
	w.lastEditAt = now
	return nil
}

func (w *TelegramStreamWriter) editCurrent(now time.Time) error {
	edit := tgbotapi.NewEditMessageText(w.chatID, w.messageID, w.content.String())
	if _, err := w.bot.Send(edit); err != nil {
		return fmt.Errorf("telegram stream edit: %w", err)
	}
	w.lastEditAt = now
	w.dirty = false
	return nil
}
