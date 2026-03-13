package tool

import (
	"context"
	"strings"
	"testing"
	"tinybot/internal/domain/model"
)

func TestMessageTool_Execute_UsesCurrentContextWhenTargetOmitted(t *testing.T) {
	var sent model.OutboundMessage
	mt := NewMessageTool(
		func(ctx context.Context, msg model.OutboundMessage) error {
			sent = msg
			return nil
		},
		model.ChannelCLI,
		"",
	)
	// TODO: 这里模拟 chat/service 在每轮开始时给 message tool 注入当前线程上下文。
	mt.SetContext(model.ChannelTelegram, "chat-42")
	got, err := mt.Execute(context.Background(), map[string]any{
		"content": "hello from tool",
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(got, "telegram:chat-42") {
		t.Fatalf("Execute() result = %q, want telegram target", got)
	}
	if sent.Channel != model.ChannelTelegram {
		t.Fatalf("sent.Channel = %q, want %q", sent.Channel, model.ChannelTelegram)
	}
	if sent.ChatID != "chat-42" {
		t.Fatalf("sent.ChatID = %q, want %q", sent.ChatID, "chat-42")
	}
	if sent.Content != "hello from tool" {
		t.Fatalf("sent.Content = %q, want %q", sent.Content, "hello from tool")
	}
}
func TestMessageTool_Execute_ReturnsReadableErrorWhenCallbackMissing(t *testing.T) {
	mt := NewMessageTool(nil, model.ChannelCLI, "direct")
	_, err := mt.Execute(context.Background(), map[string]any{
		"content": "hello",
	})
	if err == nil {
		t.Fatal("expected error when callback is missing")
	}
	if !strings.Contains(err.Error(), "callback") {
		t.Fatalf("error = %v, want callback-related error", err)
	}
}
func TestRegistry_SetMessageContextAndCallback(t *testing.T) {
	r := NewRegistry()
	r.Register(NewMessageTool(nil, model.ChannelCLI, ""))
	var sent model.OutboundMessage
	r.SetMessageContext(model.ChannelTelegram, "chat-99")
	r.SetMessageCallback(func(ctx context.Context, msg model.OutboundMessage) error {
		sent = msg
		return nil
	})
	_, err := r.Execute(context.Background(), "message", map[string]any{
		"content": "hello",
	})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if sent.Channel != model.ChannelTelegram || sent.ChatID != "chat-99" {
		t.Fatalf("sent = %#v, want telegram/chat-99", sent)
	}
}
