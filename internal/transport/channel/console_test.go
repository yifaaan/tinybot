package channel

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"tinybot/internal/domain/model"
	transportbus "tinybot/internal/transport/bus"
)

func TestConsoleChannel_StartPublishesInbound(t *testing.T) {
	t.Parallel()

	b := transportbus.NewMemoryBus(1)
	input := strings.NewReader("hello from console\n")
	var output bytes.Buffer

	channel := NewConsoleChannel(b, ConsoleChannelConfig{
		ChatID:   "gateway-chat",
		SenderID: "gateway-user",
		Prompt:   "You>",
	}, input, &output)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	defer b.Close()

	done := make(chan error, 1)
	go func() {
		done <- channel.Start(ctx)
	}()

	msg, err := b.ConsumeInbound(ctx)
	if err != nil {
		t.Fatalf("ConsumeInbound() error: %v", err)
	}
	if msg.ChatID != "gateway-chat" {
		t.Fatalf("ChatID = %q, want %q", msg.ChatID, "gateway-chat")
	}
	if msg.SenderID != "gateway-user" {
		t.Fatalf("SenderID = %q, want %q", msg.SenderID, "gateway-user")
	}
	if msg.Content != "hello from console" {
		t.Fatalf("Content = %q, want %q", msg.Content, "hello from console")
	}

	if err := <-done; err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if !strings.Contains(output.String(), "You> ") {
		t.Fatalf("output missing prompt: %q", output.String())
	}
}

func TestConsoleChannel_SendWritesReplyAndPrompt(t *testing.T) {
	t.Parallel()

	b := transportbus.NewMemoryBus(1)
	var output bytes.Buffer
	channel := NewConsoleChannel(b, ConsoleChannelConfig{
		Prompt:     "You>",
		ShowPrefix: true,
	}, strings.NewReader(""), &output)

	err := channel.Send(context.Background(), model.OutboundMessage{
		Channel: model.ChannelCLI,
		ChatID:  "gateway",
		Content: "hello back",
	})
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	text := output.String()
	if !strings.Contains(text, "tinybot> hello back") {
		t.Fatalf("output missing reply prefix: %q", text)
	}
	if !strings.HasSuffix(text, "You>") {
		t.Fatalf("output missing trailing prompt: %q", text)
	}
}

func TestConsoleChannel_Send_IgnoresEmptyMessages(t *testing.T) {
	t.Parallel()

	b := transportbus.NewMemoryBus(1)
	var output bytes.Buffer
	channel := NewConsoleChannel(b, ConsoleChannelConfig{}, strings.NewReader(""), &output)

	if err := channel.Send(context.Background(), model.OutboundMessage{Content: "   "}); err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("expected no output, got %q", output.String())
	}
}

func TestConsoleChannel_Send_StreamedMessageWritesPromptOnly(t *testing.T) {
	t.Parallel()

	b := transportbus.NewMemoryBus(1)
	var output bytes.Buffer
	channel := NewConsoleChannel(b, ConsoleChannelConfig{
		Prompt:     "You>",
		ShowPrefix: true,
	}, strings.NewReader(""), &output)

	// 先模拟 gateway 在流式过程中已经把增量文本写到了终端。
	if err := channel.WriteDelta("streamed "); err != nil {
		t.Fatalf("WriteDelta() error: %v", err)
	}
	if err := channel.WriteDelta("final answer"); err != nil {
		t.Fatalf("WriteDelta() error: %v", err)
	}

	// 流式结束后，Send 应该只补换行和提示符，
	// 而不是再次把完整内容打印一遍。
	err := channel.Send(context.Background(), model.OutboundMessage{
		Channel:  model.ChannelCLI,
		ChatID:   "gateway",
		Content:  "streamed final answer",
		Streamed: true,
	})
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	text := output.String()
	if strings.Count(text, "streamed final answer") != 1 {
		t.Fatalf("expected streamed content to appear once, got %q", text)
	}
	if strings.Contains(text, "tinybot> streamed final answer") {
		t.Fatalf("streamed reply should not be rendered again with prefix: %q", text)
	}
	if !strings.HasSuffix(text, "\nYou>") {
		t.Fatalf("output missing trailing newline and prompt: %q", text)
	}
}
