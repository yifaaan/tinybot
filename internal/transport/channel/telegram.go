package channel

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"
	"tinybot/internal/domain/model"
	"tinybot/internal/transport"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramChannel implements the Channel interface for Telegram Bot API.
// 使用 long polling 模式接收消息，通过 SendMessage API 发送回复
type TelegramChannel struct {
	bus   transport.MessageBus
	token string
	bot   *tgbotapi.BotAPI
	mu    sync.Mutex
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
		bus:   bus,
		token: token,
		bot:   bot,
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
	ucfg := tgbotapi.NewUpdate(0)
	ucfg.Timeout = 60

	updates := c.bot.GetUpdatesChan(ucfg)

	for {
		select {
		case <-ctx.Done():
			return nil
		case update, ok := <-updates:
			if !ok {
				continue
			}
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

	msg := model.InboundMessage{
		ID:        fmt.Sprintf("tg-%d", update.Message.MessageID),
		Channel:   model.ChannelTelegram,
		SenderID:  strconv.FormatInt(update.Message.From.ID, 10),
		ChatID:    strconv.FormatInt(update.Message.Chat.ID, 10),
		Content:   update.Message.Text,
		CreatedAt: time.Unix(int64(update.Message.Date), 0),
	}

	return c.bus.PublishInbound(ctx, msg)
}

func (c *TelegramChannel) Send(ctx context.Context, out model.OutboundMessage) error {
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
