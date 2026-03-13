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
