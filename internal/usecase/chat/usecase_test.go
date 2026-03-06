package chat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tinybot/internal/domain/model"
	"tinybot/internal/ports"
)

type fakeSessionRepository struct {
	sessions map[string]*model.Session
	saveErr  error
}

func newFakeSessionRepository() *fakeSessionRepository {
	return &fakeSessionRepository{
		sessions: make(map[string]*model.Session),
	}
}

func (f *fakeSessionRepository) GetSessionPath(key string) string {
	return key + ".jsonl"
}

func (f *fakeSessionRepository) GetOrCreateSession(key string) *model.Session {
	if s, ok := f.sessions[key]; ok {
		return s
	}
	s := model.NewSession(key)
	f.sessions[key] = s
	return s
}

func (f *fakeSessionRepository) LoadSession(key string) (*model.Session, error) {
	if s, ok := f.sessions[key]; ok {
		return s, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeSessionRepository) SaveSession(s *model.Session) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.sessions[s.Key] = s
	return nil
}

func (f *fakeSessionRepository) Invalidate(key string) {
	delete(f.sessions, key)
}

func (f *fakeSessionRepository) ListSessions() ([]map[string]any, error) {
	return []map[string]any{}, nil
}

type fakeLLMClient struct {
	resp             model.LLMResponse
	err              error
	capturedMessages *[]map[string]any
}

func (f fakeLLMClient) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error) {
	if f.capturedMessages != nil {
		*f.capturedMessages = cloneMessageMaps(messages)
	}
	if f.err != nil {
		return model.LLMResponse{}, f.err
	}
	return f.resp, nil
}

type fakeToolRegistry struct {
	definitions []map[string]any
	results     map[string]string
	err         error
}

func newFakeToolRegistry() *fakeToolRegistry {
	return &fakeToolRegistry{
		results: make(map[string]string),
	}
}

func (f *fakeToolRegistry) Register(tool ports.Tool) {}

func (f *fakeToolRegistry) Unregister(name string) {}

func (f *fakeToolRegistry) Get(name string) (ports.Tool, bool) {
	return nil, false
}

func (f *fakeToolRegistry) Has(name string) bool {
	return false
}

func (f *fakeToolRegistry) GetDefinitions() []map[string]any {
	return append([]map[string]any(nil), f.definitions...)
}

func (f *fakeToolRegistry) Execute(ctx context.Context, name string, params map[string]any) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if result, ok := f.results[name]; ok {
		return result, nil
	}
	return "", fmt.Errorf("tool %s not found", name)
}

func cloneMessageMaps(in []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, item := range in {
		if item == nil {
			out = append(out, nil)
			continue
		}
		cloned := make(map[string]any, len(item))
		for key, value := range item {
			cloned[key] = value
		}
		out = append(out, cloned)
	}
	return out
}

func TestNewUseCase(t *testing.T) {
	repo := newFakeSessionRepository()
	llm := fakeLLMClient{resp: model.LLMResponse{Content: "ok"}}
	tools := newFakeToolRegistry()

	if _, err := NewUseCase(nil, llm, tools, 20); err == nil {
		t.Fatal("expected error when session repository is nil")
	}
	if _, err := NewUseCase(repo, nil, tools, 20); err == nil {
		t.Fatal("expected error when llm client is nil")
	}
	if _, err := NewUseCase(repo, llm, nil, 20); err == nil {
		t.Fatal("expected error when tool registry is nil")
	}
	if _, err := NewUseCase(repo, llm, tools, 20); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUseCase_ProcessMessage(t *testing.T) {
	tests := []struct {
		name      string
		msg       model.InboundMessage
		llmResp   model.LLMResponse
		llmErr    error
		saveErr   error
		wantErr   bool
		wantReply string
	}{
		{
			name: "success",
			msg: model.InboundMessage{
				ID:      "m1",
				Channel: model.ChannelTelegram,
				ChatID:  "chat-1",
				Content: "hello",
			},
			llmResp:   model.LLMResponse{Content: "hi"},
			wantReply: "hi",
		},
		{
			name: "empty content",
			msg: model.InboundMessage{
				ID:      "m2",
				Channel: model.ChannelCLI,
				ChatID:  "local",
				Content: "   ",
			},
			wantErr: true,
		},
		{
			name: "llm error",
			msg: model.InboundMessage{
				ID:      "m3",
				Channel: model.ChannelCLI,
				ChatID:  "local",
				Content: "hi",
			},
			llmErr:  errors.New("llm failed"),
			wantErr: true,
		},
		{
			name: "save error",
			msg: model.InboundMessage{
				ID:      "m4",
				Channel: model.ChannelCLI,
				ChatID:  "local",
				Content: "hi",
			},
			llmResp: model.LLMResponse{Content: "ok"},
			saveErr: errors.New("disk failed"),
			wantErr: true,
		},
		{
			name: "empty llm content uses fallback text",
			msg: model.InboundMessage{
				ID:      "m5",
				Channel: model.ChannelCLI,
				ChatID:  "local",
				Content: "hi",
			},
			llmResp:   model.LLMResponse{Content: "  "},
			wantReply: "Sorry, I encountered an error calling the AI model.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeSessionRepository()
			repo.saveErr = tt.saveErr
			uc, err := NewUseCase(repo, fakeLLMClient{resp: tt.llmResp, err: tt.llmErr}, newFakeToolRegistry(), 20)
			if err != nil {
				t.Fatalf("NewUseCase error: %v", err)
			}

			out, err := uc.ProcessMessage(context.Background(), tt.msg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ProcessMessage error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if out.Content != tt.wantReply {
				t.Fatalf("out.Content = %q, want %q", out.Content, tt.wantReply)
			}

			session := repo.sessions[tt.msg.SessionKey()]
			if session == nil {
				t.Fatalf("session %q not found", tt.msg.SessionKey())
			}
			if len(session.Messages) != 2 {
				t.Fatalf("session message len = %d, want 2", len(session.Messages))
			}
			if session.Messages[0].Role != model.RoleUser {
				t.Fatalf("first message role = %q, want %q", session.Messages[0].Role, model.RoleUser)
			}
			if session.Messages[1].Role != model.RoleAssistant {
				t.Fatalf("second message role = %q, want %q", session.Messages[1].Role, model.RoleAssistant)
			}
		})
	}
}

func TestUseCase_ProcessDirect(t *testing.T) {
	repo := newFakeSessionRepository()
	uc, err := NewUseCase(repo, fakeLLMClient{resp: model.LLMResponse{Content: "done"}}, newFakeToolRegistry(), 20)
	if err != nil {
		t.Fatalf("NewUseCase error: %v", err)
	}

	got, err := uc.ProcessDirect(context.Background(), "cli:direct-test", "ping")
	if err != nil {
		t.Fatalf("ProcessDirect error: %v", err)
	}
	if got != "done" {
		t.Fatalf("got = %q, want %q", got, "done")
	}

	if _, ok := repo.sessions["cli:direct-test"]; !ok {
		t.Fatal("expected direct session to be saved")
	}
}

func TestUseCase_ProcessMessage_UsesContextBuilder(t *testing.T) {
	tmp := t.TempDir()
	soulPath := filepath.Join(tmp, "SOUL.md")
	if err := os.WriteFile(soulPath, []byte("Helpful and friendly"), 0o644); err != nil {
		t.Fatalf("write SOUL.md: %v", err)
	}

	var captured []map[string]any
	repo := newFakeSessionRepository()
	llm := fakeLLMClient{
		resp:             model.LLMResponse{Content: "ok"},
		capturedMessages: &captured,
	}
	uc, err := NewUseCase(repo, llm, newFakeToolRegistry(), 20)
	if err != nil {
		t.Fatalf("NewUseCase error: %v", err)
	}
	uc.SetContextBuilder(NewContextBuilder(tmp))

	_, err = uc.ProcessMessage(context.Background(), model.InboundMessage{
		ID:      "m-system",
		Channel: model.ChannelCLI,
		ChatID:  "local",
		Content: "ping",
	})
	if err != nil {
		t.Fatalf("ProcessMessage error: %v", err)
	}

	if len(captured) < 2 {
		t.Fatalf("captured messages len = %d, want at least 2", len(captured))
	}
	if captured[0]["role"] != "system" {
		t.Fatalf("first role = %#v, want %q", captured[0]["role"], "system")
	}
	content, _ := captured[0]["content"].(string)
	if !strings.Contains(content, "Helpful and friendly") {
		t.Fatalf("system prompt does not include workspace bootstrap content: %q", content)
	}
	if captured[1]["role"] != model.RoleUser {
		t.Fatalf("second role = %#v, want %q", captured[1]["role"], model.RoleUser)
	}
}

func TestToLLMMessages(t *testing.T) {
	now := model.NewSession("tmp").CreatedAt
	history := []*model.Message{
		nil,
		{
			Role:      model.RoleUser,
			Content:   "hello",
			CreatedAt: now,
		},
		{
			Role:       model.RoleTool,
			Content:    "result",
			CreatedAt:  now,
			ToolCallID: "call-1",
			Name:       "search",
			ToolCalls:  map[string]any{"n": 1},
		},
	}

	got := toLLMMessages(history)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0]["role"] != model.RoleUser || got[0]["content"] != "hello" {
		t.Fatalf("unexpected first message: %#v", got[0])
	}
	if got[1]["tool_call_id"] != "call-1" {
		t.Fatalf("tool_call_id = %#v, want %q", got[1]["tool_call_id"], "call-1")
	}
	if got[1]["name"] != "search" {
		t.Fatalf("name = %#v, want %q", got[1]["name"], "search")
	}
}
