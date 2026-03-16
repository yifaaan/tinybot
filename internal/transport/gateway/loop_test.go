package gateway

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"tinybot/internal/domain/model"
	transportbus "tinybot/internal/transport/bus"
)

type fakeProcessor struct {
	out  model.OutboundMessage
	err  error
	seen []model.InboundMessage
	mu   sync.Mutex
}

func (p *fakeProcessor) ProcessMessage(_ context.Context, msg model.InboundMessage) (model.OutboundMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.seen = append(p.seen, msg)
	return p.out, p.err
}

func (p *fakeProcessor) Seen() []model.InboundMessage {
	p.mu.Lock()
	defer p.mu.Unlock()

	cp := make([]model.InboundMessage, len(p.seen))
	copy(cp, p.seen)
	return cp
}

type fakeStreamingProcessor struct {
	streamOut    model.OutboundMessage
	streamErr    error
	streamDeltas []string

	processCalls int
	streamCalls  int
	seen         []model.InboundMessage
	mu           sync.Mutex
}

func (p *fakeStreamingProcessor) ProcessMessage(_ context.Context, msg model.InboundMessage) (model.OutboundMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.processCalls++
	p.seen = append(p.seen, msg)
	return model.OutboundMessage{}, nil
}

func (p *fakeStreamingProcessor) ProcessMessageStream(_ context.Context, msg model.InboundMessage, onDelta func(string), onThinking func(string)) (model.OutboundMessage, error) {
	p.mu.Lock()
	p.streamCalls++
	p.seen = append(p.seen, msg)
	deltas := append([]string(nil), p.streamDeltas...)
	out := p.streamOut
	err := p.streamErr
	p.mu.Unlock()

	for _, delta := range deltas {
		if onDelta != nil {
			onDelta(delta)
		}
	}

	return out, err
}

func (p *fakeStreamingProcessor) Calls() (process int, stream int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.processCalls, p.streamCalls
}

type fakeStreamWriter struct {
	deltas []string
	mu     sync.Mutex
}

func (w *fakeStreamWriter) WriteDelta(delta string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.deltas = append(w.deltas, delta)
	return nil
}

func (w *fakeStreamWriter) Deltas() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := make([]string, len(w.deltas))
	copy(out, w.deltas)
	return out
}

func TestLoop_ForwardsInboundToOutbound(t *testing.T) {
	t.Parallel()

	b := transportbus.NewMemoryBus(16)
	processor := &fakeProcessor{
		out: model.OutboundMessage{
			Channel: "test_channel",
			ChatID:  "test_chat_id",
			Content: "test response",
		},
	}
	loop := NewLoop(processor, b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer b.Close()

	done := make(chan error, 1)
	go func() {
		done <- loop.Run(ctx)
	}()

	inbound := model.InboundMessage{
		ID:      "msg-1",
		Channel: model.ChannelCLI,
		ChatID:  "direct",
		Content: "hello",
	}

	if err := b.PublishInbound(ctx, inbound); err != nil {
		t.Fatalf("publish inbound: %v", err)
	}
	outboundCtx, outboundCancel := context.WithTimeout(ctx, 2*time.Second)
	defer outboundCancel()

	got, err := b.ConsumeOutbound(outboundCtx)
	if err != nil {
		t.Fatalf("ConsumeOutbound() error: %v", err)
	}

	if got.Content != processor.out.Content {
		t.Fatalf("expected content %q, got %q", processor.out.Content, got.Content)
	}

	_ = done
}

func TestLoop_PublishesFallbackWhenProcessorFails(t *testing.T) {
	t.Parallel()

	b := transportbus.NewMemoryBus(1)
	processor := &fakeProcessor{
		err: errors.New("boom"),
	}

	loop := NewLoop(processor, b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer b.Close()

	go func() {
		_ = loop.Run(ctx)
	}()

	inbound := model.InboundMessage{
		ID:      "msg-2",
		Channel: model.ChannelCLI,
		ChatID:  "direct",
		Content: "trigger error",
	}

	if err := b.PublishInbound(ctx, inbound); err != nil {
		t.Fatalf("PublishInbound() error: %v", err)
	}

	outboundCtx, outboundCancel := context.WithTimeout(ctx, 2*time.Second)
	defer outboundCancel()

	out, err := b.ConsumeOutbound(outboundCtx)
	if err != nil {
		t.Fatalf("ConsumeOutbound() error: %v", err)
	}
	if !strings.Contains(out.Content, "boom") {
		t.Fatalf("expected fallback containing boom, got %q", out.Content)
	}
}

func TestLoop_UsesStreamingProcessorWhenStreamWriterPresent(t *testing.T) {
	t.Parallel()

	b := transportbus.NewMemoryBus(1)
	writer := &fakeStreamWriter{}
	processor := &fakeStreamingProcessor{
		streamOut: model.OutboundMessage{
			Channel: model.ChannelCLI,
			ChatID:  "direct",
			Content: "streamed final answer",
		},
		streamDeltas: []string{"streamed ", "final ", "answer"},
	}

	loop := NewLoop(processor, b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer b.Close()

	go func() {
		_ = loop.Run(ctx)
	}()

	inbound := model.InboundMessage{
		ID:           "msg-stream-1",
		Channel:      model.ChannelCLI,
		ChatID:       "direct",
		Content:      "hello",
		StreamWriter: writer,
	}

	if err := b.PublishInbound(ctx, inbound); err != nil {
		t.Fatalf("PublishInbound() error: %v", err)
	}

	outboundCtx, outboundCancel := context.WithTimeout(ctx, 2*time.Second)
	defer outboundCancel()

	out, err := b.ConsumeOutbound(outboundCtx)
	if err != nil {
		t.Fatalf("ConsumeOutbound() error: %v", err)
	}

	processCalls, streamCalls := processor.Calls()
	if processCalls != 0 {
		t.Fatalf("processCalls = %d, want %d", processCalls, 0)
	}
	if streamCalls != 1 {
		t.Fatalf("streamCalls = %d, want %d", streamCalls, 1)
	}

	if got := strings.Join(writer.Deltas(), ""); got != "streamed final answer" {
		t.Fatalf("joined deltas = %q, want %q", got, "streamed final answer")
	}

	if out.Content != "streamed final answer" {
		t.Fatalf("out.Content = %q, want %q", out.Content, "streamed final answer")
	}
	if !out.Streamed {
		t.Fatal("out.Streamed = false, want true")
	}
}

func TestLoop_StreamingWithoutDeltaKeepsStreamedFalse(t *testing.T) {
	t.Parallel()

	b := transportbus.NewMemoryBus(1)
	writer := &fakeStreamWriter{}
	processor := &fakeStreamingProcessor{
		streamOut: model.OutboundMessage{
			Channel: model.ChannelCLI,
			ChatID:  "direct",
			Content: "final without delta",
		},
	}

	loop := NewLoop(processor, b)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer b.Close()

	go func() {
		_ = loop.Run(ctx)
	}()

	inbound := model.InboundMessage{
		ID:           "msg-stream-2",
		Channel:      model.ChannelCLI,
		ChatID:       "direct",
		Content:      "hello",
		StreamWriter: writer,
	}

	if err := b.PublishInbound(ctx, inbound); err != nil {
		t.Fatalf("PublishInbound() error: %v", err)
	}

	outboundCtx, outboundCancel := context.WithTimeout(ctx, 2*time.Second)
	defer outboundCancel()

	out, err := b.ConsumeOutbound(outboundCtx)
	if err != nil {
		t.Fatalf("ConsumeOutbound() error: %v", err)
	}

	processCalls, streamCalls := processor.Calls()
	if processCalls != 0 {
		t.Fatalf("processCalls = %d, want %d", processCalls, 0)
	}
	if streamCalls != 1 {
		t.Fatalf("streamCalls = %d, want %d", streamCalls, 1)
	}

	if len(writer.Deltas()) != 0 {
		t.Fatalf("writer deltas len = %d, want 0", len(writer.Deltas()))
	}

	if out.Content != "final without delta" {
		t.Fatalf("out.Content = %q, want %q", out.Content, "final without delta")
	}
	if out.Streamed {
		t.Fatal("out.Streamed = true, want false")
	}
}
