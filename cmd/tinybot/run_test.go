package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tinybot/internal/domain/model"
)

type fakeDirectChatProcessor struct {
	reply string
	err   error
	last  struct {
		sessionKey string
		content    string
	}
}

func (f *fakeDirectChatProcessor) ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error) {
	f.last.sessionKey = sessionKey
	f.last.content = content
	if f.err != nil {
		return "", f.err
	}
	return f.reply, nil
}

type fakeDirectStreamingProcessor struct {
	streamOut    model.OutboundMessage
	streamErr    error
	streamDeltas []string

	directCalls int
	streamCalls int
	lastMsg     model.InboundMessage
}

func (f *fakeDirectStreamingProcessor) ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error) {
	// 这条测试期望 run() 命中流式分支；
	// 如果这里被调用，说明 direct chat 错误地回退到了非流式路径。
	f.directCalls++
	return "", nil
}

func (f *fakeDirectStreamingProcessor) ProcessMessageStream(ctx context.Context, msg model.InboundMessage, onDelta func(string)) (model.OutboundMessage, error) {
	f.streamCalls++
	f.lastMsg = msg

	// 逐个推送增量文本，模拟真实 CLI 直连 chat service 时的流式输出。
	for _, delta := range f.streamDeltas {
		if onDelta != nil {
			onDelta(delta)
		}
	}

	return f.streamOut, f.streamErr
}

func TestRun_NoArgs_PrintsHelp(t *testing.T) {
	var out bytes.Buffer

	if err := run(nil, &out, filepath.Join(t.TempDir(), "workspace")); err != nil {
		t.Fatalf("run() error: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "Usage: tinybot") {
		t.Fatalf("help output missing usage: %s", text)
	}
	if !strings.Contains(text, "onboard") {
		t.Fatalf("help output missing onboard command: %s", text)
	}
	if !strings.Contains(text, "status") {
		t.Fatalf("help output missing status command: %s", text)
	}
}

func TestRun_Onboard_CreatesWorkspace(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	var out bytes.Buffer

	if err := run([]string{"onboard"}, &out, workspace); err != nil {
		t.Fatalf("run(onboard) error: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "Workspace ready at") {
		t.Fatalf("onboard output missing workspace message: %s", text)
	}
	if !strings.Contains(text, "Created files:") {
		t.Fatalf("onboard output missing created files section: %s", text)
	}
}

func TestRun_Status_ShowsMissingFilesBeforeOnboard(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	var out bytes.Buffer

	if err := run([]string{"status"}, &out, workspace); err != nil {
		t.Fatalf("run(status) error: %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "Workspace exists: false") {
		t.Fatalf("status output missing workspace flag: %s", text)
	}
	if !strings.Contains(text, "Missing files:") {
		t.Fatalf("status output missing missing-files section: %s", text)
	}
	if !strings.Contains(text, "config.json") {
		t.Fatalf("status output missing config.json: %s", text)
	}
	if !strings.Contains(text, filepath.Join("memory", "MEMORY.md")) {
		t.Fatalf("status output missing memory file: %s", text)
	}
}

func TestRun_DirectChat_UsesChatService(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	var out bytes.Buffer
	processor := &fakeDirectChatProcessor{reply: "pong"}

	oldFactory := newDirectChatProcessor
	newDirectChatProcessor = func(workspace string) (directChatProcessor, error) {
		return processor, nil
	}
	defer func() {
		newDirectChatProcessor = oldFactory
	}()

	if err := run([]string{"ping"}, &out, workspace); err != nil {
		t.Fatalf("run(direct chat) error: %v", err)
	}
	if out.String() != "pong\n" {
		t.Fatalf("direct chat output = %q, want %q", out.String(), "pong\n")
	}
	if processor.last.sessionKey != "cli:direct" {
		t.Fatalf("sessionKey = %q, want %q", processor.last.sessionKey, "cli:direct")
	}
	if processor.last.content != "ping" {
		t.Fatalf("content = %q, want %q", processor.last.content, "ping")
	}
}

func TestRun_DirectChat_UsesStreamingProcessor(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	var out bytes.Buffer
	processor := &fakeDirectStreamingProcessor{
		streamDeltas: []string{"pon", "g"},
		streamOut: model.OutboundMessage{
			Channel: model.ChannelCLI,
			ChatID:  "direct",
			Content: "pong",
		},
	}

	oldFactory := newDirectChatProcessor
	newDirectChatProcessor = func(workspace string) (directChatProcessor, error) {
		return processor, nil
	}
	defer func() {
		newDirectChatProcessor = oldFactory
	}()

	if err := run([]string{"ping"}, &out, workspace); err != nil {
		t.Fatalf("run(direct streaming chat) error: %v", err)
	}

	// 终端应该只看到流式 delta 拼接后的结果，
	// run() 在流结束后只补一个换行，不应该再把完整答案打印第二次。
	if out.String() != "pong\n" {
		t.Fatalf("direct streaming output = %q, want %q", out.String(), "pong\n")
	}

	if processor.directCalls != 0 {
		t.Fatalf("directCalls = %d, want %d", processor.directCalls, 0)
	}
	if processor.streamCalls != 1 {
		t.Fatalf("streamCalls = %d, want %d", processor.streamCalls, 1)
	}

	if processor.lastMsg.Channel != model.ChannelCLI {
		t.Fatalf("channel = %q, want %q", processor.lastMsg.Channel, model.ChannelCLI)
	}
	if processor.lastMsg.ChatID != "direct" {
		t.Fatalf("chatID = %q, want %q", processor.lastMsg.ChatID, "direct")
	}
	if processor.lastMsg.Content != "ping" {
		t.Fatalf("content = %q, want %q", processor.lastMsg.Content, "ping")
	}
	if got := processor.lastMsg.SessionKey(); got != "cli:direct" {
		t.Fatalf("sessionKey = %q, want %q", got, "cli:direct")
	}
}

func TestRun_DirectChat_WrapsProcessorError(t *testing.T) {
	var out bytes.Buffer
	processor := &fakeDirectChatProcessor{err: errors.New("boom")}

	oldFactory := newDirectChatProcessor
	newDirectChatProcessor = func(workspace string) (directChatProcessor, error) {
		return processor, nil
	}
	defer func() {
		newDirectChatProcessor = oldFactory
	}()

	err := run([]string{"ping"}, &out, filepath.Join(t.TempDir(), "workspace"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to process direct message") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_CronList_Empty(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	var out bytes.Buffer
	if err := run([]string{"cron", "list"}, &out, workspace); err != nil {
		t.Fatalf("run(cron list) error: %v", err)
	}
	if !strings.Contains(out.String(), "No cron jobs found.") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}
func TestRun_CronAdd_ThenList(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	var out bytes.Buffer
	if err := run([]string{"cron", "add", "daily", "60", "check status"}, &out, workspace); err != nil {
		t.Fatalf("run(cron add) error: %v", err)
	}
	out.Reset()
	if err := run([]string{"cron", "list"}, &out, workspace); err != nil {
		t.Fatalf("run(cron list) error: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "daily") {
		t.Fatalf("cron list output missing job name: %s", text)
	}
}
func TestRun_CronAdd_InvalidArgs(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	var out bytes.Buffer
	err := run([]string{"cron", "add", "daily"}, &out, workspace)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRun_CronAddAt_ThenList(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "workspace")
	var out bytes.Buffer

	at := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)

	if err := run([]string{"cron", "add-at", "meeting", at, "join standup"}, &out, workspace); err != nil {
		t.Fatalf("run(cron add-at) error: %v", err)
	}

	out.Reset()

	if err := run([]string{"cron", "list"}, &out, workspace); err != nil {
		t.Fatalf("run(cron list) error: %v", err)
	}

	text := out.String()

	if !strings.Contains(text, "meeting") {
		t.Fatalf("cron list output missing job name: %s", text)
	}
	if !strings.Contains(text, "at=") {
		t.Fatalf("cron list output missing at= field: %s", text)
	}
}
