package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"tinybot/internal/domain/model"
)

type SendMessageCallback func(ctx context.Context, msg model.OutboundMessage) error

// MessageTool is a tool that allows the agent to send messages to the user through the chat channel(Telegram, QQ).
type MessageTool struct {
	sendMessageCallback SendMessageCallback
	defaultChannel      model.Channel
	defaultChatID       string
}

func NewMessageTool(sendMessageCallback SendMessageCallback, defaultChannel model.Channel, defaultChatID string) *MessageTool {
	return &MessageTool{
		sendMessageCallback: sendMessageCallback,
		defaultChannel:      defaultChannel,
		defaultChatID:       defaultChatID,
	}
}

func (t *MessageTool) SetContext(channel model.Channel, chatID string) {
	t.defaultChannel = channel
	t.defaultChatID = chatID
}

// SetSendMessageCallback 把真正的外发动作接到 transport bus 或其他发送器上
func (t *MessageTool) SetSendMessageCallback(callback SendMessageCallback) {
	t.sendMessageCallback = callback
}

func (t *MessageTool) Name() string {
	return "message"
}

func (t *MessageTool) Description() string {
	return "Send a message to the user. Use this when you want to communicate something."
}

func (t *MessageTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"content": {
                    "type": "string",
                    "description": "The message content to send"
                },
                "channel": {
                    "type": "string",
                    "description": "Optional: target channel (telegram, discord, etc.)"
                },
                "chat_id": {
                    "type": "string",
                    "description": "Optional: target chat/user ID"
                }
			},
			"required": ["content"]
		}`),
	}
}

func (t *MessageTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	content, ok := params["content"].(string)
	if !ok || strings.TrimSpace(content) == "" {
		return "", errors.New("send message: content is required")
	}
	channel, ok := params["channel"].(string)
	if !ok || channel == "" {
		channel = string(t.defaultChannel)
	}
	chatID, ok := params["chat_id"].(string)
	if !ok || chatID == "" {
		chatID = t.defaultChatID
	}
	if channel == "" || chatID == "" {
		return "", errors.New("send message: channel or chatID is empty")
	}

	// direct chat模式下没有OutboundMessage bus
	if t.sendMessageCallback == nil {
		return "", errors.New("send message: callback is not configured")
	}
	msg := model.OutboundMessage{
		Channel: model.Channel(channel),
		ChatID:  chatID,
		Content: params["content"].(string),
	}
	err := t.sendMessageCallback(ctx, msg)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Message sent to %s:%s", channel, chatID), nil
}
