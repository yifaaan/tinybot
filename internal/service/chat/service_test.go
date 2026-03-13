package chat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	workspaceadapter "tinybot/internal/adapters/workspace"
	"tinybot/internal/domain/model"
)

type contextKey string

const repositoryContextKey contextKey = "repository"

type fakeSessionRepository struct {
	sessions    map[string]*model.Session
	getErr      error
	saveErr     error
	getCalls    int
	saveCalls   int
	lastGetCtx  context.Context
	lastSaveCtx context.Context
}

func newFakeSessionRepository() *fakeSessionRepository {
	return &fakeSessionRepository{sessions: make(map[string]*model.Session)}
}

func (f *fakeSessionRepository) GetOrCreateSession(ctx context.Context, key string) (*model.Session, error) {
	f.getCalls++
	f.lastGetCtx = ctx
	if f.getErr != nil {
		return nil, f.getErr
	}
	if session, ok := f.sessions[key]; ok {
		return session, nil
	}
	session := model.NewSession(key)
	f.sessions[key] = session
	return session, nil
}

func (f *fakeSessionRepository) SaveSession(ctx context.Context, session *model.Session) error {
	f.saveCalls++
	f.lastSaveCtx = ctx
	if f.saveErr != nil {
		return f.saveErr
	}
	f.sessions[session.Key] = session
	return nil
}

type fakeLLMClient struct {
	resp             model.LLMResponse
	err              error
	capturedMessages *[]map[string]any
	state            *fakeLLMState
}

func (f fakeLLMClient) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error) {
	if f.capturedMessages != nil {
		*f.capturedMessages = cloneMessageMaps(messages)
	}
	if f.state != nil {
		f.state.calls = append(f.state.calls, fakeLLMCall{
			messages:    cloneMessageMaps(messages),
			tools:       cloneMessageMaps(tools),
			maxTokens:   maxTokens,
			temperature: temperature,
		})
		idx := len(f.state.calls) - 1
		if idx < len(f.state.errs) && f.state.errs[idx] != nil {
			return model.LLMResponse{}, f.state.errs[idx]
		}
		if idx < len(f.state.responses) {
			return f.state.responses[idx], nil
		}
	}
	if f.err != nil {
		return model.LLMResponse{}, f.err
	}
	return f.resp, nil
}

type fakeLLMCall struct {
	messages    []map[string]any
	tools       []map[string]any
	maxTokens   int
	temperature float32
}

type fakeLLMState struct {
	responses []model.LLMResponse
	errs      []error
	calls     []fakeLLMCall
}

func newFakeLLMSequence(responses ...model.LLMResponse) (*fakeLLMClient, *fakeLLMState) {
	state := &fakeLLMState{responses: append([]model.LLMResponse(nil), responses...)}
	return &fakeLLMClient{state: state}, state
}

type fakeToolRegistry struct {
	definitions []map[string]any
	results     map[string]string
	err         error
	executions  []fakeToolExecution

	messageChannel model.Channel
	messageChatID  string
}

type capturePromptBuilder struct {
	lastSkillNames []string
}

func (b *capturePromptBuilder) BuildMessages(history []*model.Message, currentMessage string, skillNames []string) []map[string]any {
	_ = history
	b.lastSkillNames = append([]string(nil), skillNames...)
	return []map[string]any{
		{"role": model.RoleSystem, "content": "captured"},
		{"role": model.RoleUser, "content": currentMessage},
	}
}

func (b *capturePromptBuilder) AddAssistantMessage(messages []map[string]any, content string, toolCalls []map[string]any) []map[string]any {
	msg := map[string]any{"role": model.RoleAssistant}
	if content != "" {
		msg["content"] = content
	}
	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
	}
	return append(messages, msg)
}

func (b *capturePromptBuilder) AddToolResult(messages []map[string]any, toolCallID string, toolName string, result string) []map[string]any {
	return append(messages, map[string]any{
		"role":         model.RoleTool,
		"tool_call_id": toolCallID,
		"name":         toolName,
		"content":      result,
	})
}

func newFakeToolRegistry() *fakeToolRegistry {
	return &fakeToolRegistry{results: make(map[string]string)}
}

func (f *fakeToolRegistry) GetDefinitions() []map[string]any {
	return append([]map[string]any(nil), f.definitions...)
}

// SetMessageContext 让 fakeToolRegistry 满足 service.go 里那个可选窄接口。
// 这样我们就能在不引入真实 Registry 的前提下测试 service 的 wiring。
func (f *fakeToolRegistry) SetMessageContext(channel model.Channel, chatID string) {
	f.messageChannel = channel
	f.messageChatID = chatID
}
func TestService_ProcessMessage_SetsMessageToolContext(t *testing.T) {
	repo := newFakeSessionRepository()
	tools := newFakeToolRegistry()
	service, err := NewService(
		repo,
		fakeLLMClient{resp: model.LLMResponse{Content: "ok"}},
		tools,
		newTestPromptBuilder(t.TempDir()),
		20,
		8192,
		0.7,
	)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}
	_, err = service.ProcessMessage(context.Background(), model.InboundMessage{
		ID:      "m-context",
		Channel: model.ChannelTelegram,
		ChatID:  "chat-ctx",
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("ProcessMessage() error: %v", err)
	}
	if tools.messageChannel != model.ChannelTelegram {
		t.Fatalf("messageChannel = %q, want %q", tools.messageChannel, model.ChannelTelegram)
	}
	if tools.messageChatID != "chat-ctx" {
		t.Fatalf("messageChatID = %q, want %q", tools.messageChatID, "chat-ctx")
	}
}

func (f *fakeToolRegistry) Execute(ctx context.Context, name string, params map[string]any) (string, error) {
	f.executions = append(f.executions, fakeToolExecution{name: name, params: cloneMap(params)})
	if f.err != nil {
		return "", f.err
	}
	if result, ok := f.results[name]; ok {
		return result, nil
	}
	return "", fmt.Errorf("tool %s not found", name)
}

type fakeToolExecution struct {
	name   string
	params map[string]any
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
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

func newTestPromptBuilder(workspace string) *Builder {
	return NewPromptBuilder(
		workspace,
		workspaceadapter.NewBootstrapReader(workspace),
		workspaceadapter.NewMemoryStore(workspace),
		workspaceadapter.NewSkillsLoader(workspace, ""),
	)
}

func TestNewService(t *testing.T) {
	tests := []struct {
		name    string
		repo    SessionRepository
		llm     CompletionClient
		tools   ToolExecutor
		prompts PromptBuilder
		wantErr bool
	}{
		{
			name:    "requires session repository",
			llm:     fakeLLMClient{resp: model.LLMResponse{Content: "ok"}},
			tools:   newFakeToolRegistry(),
			prompts: newTestPromptBuilder(t.TempDir()),
			wantErr: true,
		},
		{
			name:    "requires llm client",
			repo:    newFakeSessionRepository(),
			tools:   newFakeToolRegistry(),
			prompts: newTestPromptBuilder(t.TempDir()),
			wantErr: true,
		},
		{
			name:    "requires tool executor",
			repo:    newFakeSessionRepository(),
			llm:     fakeLLMClient{resp: model.LLMResponse{Content: "ok"}},
			prompts: newTestPromptBuilder(t.TempDir()),
			wantErr: true,
		},
		{
			name:    "requires prompt builder",
			repo:    newFakeSessionRepository(),
			llm:     fakeLLMClient{resp: model.LLMResponse{Content: "ok"}},
			tools:   newFakeToolRegistry(),
			wantErr: true,
		},
		{
			name:    "accepts full dependency set",
			repo:    newFakeSessionRepository(),
			llm:     fakeLLMClient{resp: model.LLMResponse{Content: "ok"}},
			tools:   newFakeToolRegistry(),
			prompts: newTestPromptBuilder(t.TempDir()),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService(tt.repo, tt.llm, tt.tools, tt.prompts, 20, 8192, 0.7)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewService() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_ProcessMessage(t *testing.T) {
	tests := []struct {
		name          string
		msg           model.InboundMessage
		llmResp       model.LLMResponse
		llmErr        error
		getErr        error
		saveErr       error
		wantErr       bool
		wantReply     string
		wantGetCalls  int
		wantSaveCalls int
	}{
		{
			name: "success",
			msg: model.InboundMessage{
				ID:      "m1",
				Channel: model.ChannelTelegram,
				ChatID:  "chat-1",
				Content: "hello",
			},
			llmResp:       model.LLMResponse{Content: "hi"},
			wantReply:     "hi",
			wantGetCalls:  1,
			wantSaveCalls: 1,
		},
		{
			name: "empty content",
			msg: model.InboundMessage{
				ID:      "m2",
				Channel: model.ChannelCLI,
				ChatID:  "local",
				Content: "   ",
			},
			wantErr:       true,
			wantGetCalls:  0,
			wantSaveCalls: 0,
		},
		{
			name: "llm error",
			msg: model.InboundMessage{
				ID:      "m3",
				Channel: model.ChannelCLI,
				ChatID:  "local",
				Content: "hi",
			},
			llmErr:        errors.New("llm failed"),
			wantErr:       true,
			wantGetCalls:  1,
			wantSaveCalls: 0,
		},
		{
			name: "session repository get error",
			msg: model.InboundMessage{
				ID:      "m3b",
				Channel: model.ChannelCLI,
				ChatID:  "local",
				Content: "hi",
			},
			getErr:        errors.New("load failed"),
			wantErr:       true,
			wantGetCalls:  1,
			wantSaveCalls: 0,
		},
		{
			name: "save error",
			msg: model.InboundMessage{
				ID:      "m4",
				Channel: model.ChannelCLI,
				ChatID:  "local",
				Content: "hi",
			},
			llmResp:       model.LLMResponse{Content: "ok"},
			saveErr:       errors.New("disk failed"),
			wantErr:       true,
			wantGetCalls:  1,
			wantSaveCalls: 1,
		},
		{
			name: "empty llm content uses fallback text",
			msg: model.InboundMessage{
				ID:      "m5",
				Channel: model.ChannelCLI,
				ChatID:  "local",
				Content: "hi",
			},
			llmResp:       model.LLMResponse{Content: "  "},
			wantReply:     "Sorry, I encountered an error calling the AI model.",
			wantGetCalls:  1,
			wantSaveCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeSessionRepository()
			repo.getErr = tt.getErr
			repo.saveErr = tt.saveErr
			service, err := NewService(repo, fakeLLMClient{resp: tt.llmResp, err: tt.llmErr}, newFakeToolRegistry(), newTestPromptBuilder(t.TempDir()), 20, 8192, 0.7)
			if err != nil {
				t.Fatalf("NewService error: %v", err)
			}

			ctx := context.WithValue(context.Background(), repositoryContextKey, tt.name)
			out, err := service.ProcessMessage(ctx, tt.msg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ProcessMessage error = %v, wantErr = %v", err, tt.wantErr)
			}
			if repo.getCalls != tt.wantGetCalls {
				t.Fatalf("getCalls = %d, want %d", repo.getCalls, tt.wantGetCalls)
			}
			if repo.saveCalls != tt.wantSaveCalls {
				t.Fatalf("saveCalls = %d, want %d", repo.saveCalls, tt.wantSaveCalls)
			}
			if tt.wantGetCalls > 0 {
				if got := repo.lastGetCtx.Value(repositoryContextKey); got != tt.name {
					t.Fatalf("get context value = %#v, want %q", got, tt.name)
				}
			}
			if tt.wantSaveCalls > 0 {
				if got := repo.lastSaveCtx.Value(repositoryContextKey); got != tt.name {
					t.Fatalf("save context value = %#v, want %q", got, tt.name)
				}
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

func TestService_ProcessDirect_UsesExplicitSessionKey(t *testing.T) {
	repo := newFakeSessionRepository()
	service, err := NewService(repo, fakeLLMClient{resp: model.LLMResponse{Content: "done"}}, newFakeToolRegistry(), newTestPromptBuilder(t.TempDir()), 20, 8192, 0.7)
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	ctx := context.WithValue(context.Background(), repositoryContextKey, "direct")
	got, err := service.ProcessDirect(ctx, "cli:direct-test", "ping")
	if err != nil {
		t.Fatalf("ProcessDirect error: %v", err)
	}
	if got != "done" {
		t.Fatalf("got = %q, want %q", got, "done")
	}

	if _, ok := repo.sessions["cli:direct-test"]; !ok {
		t.Fatal("expected direct session to be saved")
	}
	if got := repo.lastGetCtx.Value(repositoryContextKey); got != "direct" {
		t.Fatalf("get context value = %#v, want %q", got, "direct")
	}
	if got := repo.lastSaveCtx.Value(repositoryContextKey); got != "direct" {
		t.Fatalf("save context value = %#v, want %q", got, "direct")
	}
}

func TestService_ProcessMessage_UsesPromptBuilder(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "SOUL.md"), []byte("Helpful and friendly"), 0o644); err != nil {
		t.Fatalf("write SOUL.md: %v", err)
	}

	var captured []map[string]any
	repo := newFakeSessionRepository()
	llm := fakeLLMClient{
		resp:             model.LLMResponse{Content: "ok"},
		capturedMessages: &captured,
	}
	service, err := NewService(repo, llm, newFakeToolRegistry(), newTestPromptBuilder(tmp), 20, 8192, 0.7)
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	_, err = service.ProcessMessage(context.Background(), model.InboundMessage{
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
	if captured[0]["role"] != model.RoleSystem {
		t.Fatalf("first role = %#v, want %q", captured[0]["role"], model.RoleSystem)
	}
	content, _ := captured[0]["content"].(string)
	if !strings.Contains(content, "Helpful and friendly") {
		t.Fatalf("system prompt does not include workspace bootstrap content: %q", content)
	}
	if captured[1]["role"] != model.RoleUser {
		t.Fatalf("second role = %#v, want %q", captured[1]["role"], model.RoleUser)
	}
}

func TestService_ProcessMessage_ForwardsSelectedSkillsToPromptBuilder(t *testing.T) {
	repo := newFakeSessionRepository()
	builder := &capturePromptBuilder{}
	service, err := NewService(
		repo,
		fakeLLMClient{resp: model.LLMResponse{Content: "ok"}},
		newFakeToolRegistry(),
		builder,
		20,
		8192,
		0.7,
	)
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	_, err = service.ProcessMessage(context.Background(), model.InboundMessage{
		ID:             "m-selected-skills",
		Channel:        model.ChannelCLI,
		ChatID:         "local",
		Content:        "use selected skill",
		SelectedSkills: []string{"foo", "bar"},
	})
	if err != nil {
		t.Fatalf("ProcessMessage error: %v", err)
	}

	if got := strings.Join(builder.lastSkillNames, ","); got != "foo,bar" {
		t.Fatalf("selected skills = %q, want %q", got, "foo,bar")
	}
}

func TestService_ProcessMessage_DoesNotDuplicateCurrentUserMessage(t *testing.T) {
	var captured []map[string]any
	repo := newFakeSessionRepository()
	llm := fakeLLMClient{
		resp:             model.LLMResponse{Content: "ok"},
		capturedMessages: &captured,
	}
	service, err := NewService(repo, llm, newFakeToolRegistry(), newTestPromptBuilder(t.TempDir()), 20, 8192, 0.7)
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	msg := model.InboundMessage{
		ID:      "m-no-dup",
		Channel: model.ChannelCLI,
		ChatID:  "local",
		Content: "unique current message",
	}
	_, err = service.ProcessMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("ProcessMessage error: %v", err)
	}

	count := 0
	for _, item := range captured {
		role, _ := item["role"].(string)
		content, _ := item["content"].(string)
		if role == model.RoleUser && content == msg.Content {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("current user message count = %d, want 1; messages = %#v", count, captured)
	}
}

func TestService_ProcessMessage_UsesConfiguredTokenAndTemperature(t *testing.T) {
	llm, state := newFakeLLMSequence(model.LLMResponse{Content: "ok"})
	repo := newFakeSessionRepository()
	service, err := NewService(repo, llm, newFakeToolRegistry(), newTestPromptBuilder(t.TempDir()), 7, 321, 0.25)
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	_, err = service.ProcessMessage(context.Background(), model.InboundMessage{
		ID:      "m-max-tokens",
		Channel: model.ChannelCLI,
		ChatID:  "local",
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("ProcessMessage error: %v", err)
	}

	if len(state.calls) != 1 {
		t.Fatalf("llm call count = %d, want 1", len(state.calls))
	}
	if state.calls[0].maxTokens != 321 {
		t.Fatalf("maxTokens = %d, want %d", state.calls[0].maxTokens, 321)
	}
	if state.calls[0].temperature != 0.25 {
		t.Fatalf("temperature = %v, want %v", state.calls[0].temperature, 0.25)
	}
}

func TestService_ProcessMessage_ToolLoopRegression(t *testing.T) {
	llm, state := newFakeLLMSequence(
		model.LLMResponse{
			Content: "Let me inspect the file.",
			ToolCalls: []*model.ToolCall{
				{
					ID:   "call-1",
					Name: "read_file",
					Args: map[string]any{"path": "README.md"},
				},
			},
		},
		model.LLMResponse{
			Content: "Done.",
		},
	)
	repo := newFakeSessionRepository()
	tools := newFakeToolRegistry()
	tools.results["read_file"] = "README content"
	tools.definitions = []map[string]any{
		{
			"type": "function",
			"function": map[string]any{
				"name":        "read_file",
				"description": "Read a file",
				"parameters": map[string]any{
					"type": "object",
				},
			},
		},
	}

	service, err := NewService(repo, llm, tools, newTestPromptBuilder(t.TempDir()), 20, 8192, 0.7)
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	msg := model.InboundMessage{
		ID:      "m-tool-loop",
		Channel: model.ChannelCLI,
		ChatID:  "local",
		Content: "read the file",
	}
	out, err := service.ProcessMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("ProcessMessage error: %v", err)
	}
	if out.Content != "Done." {
		t.Fatalf("out.Content = %q, want %q", out.Content, "Done.")
	}

	if len(state.calls) != 2 {
		t.Fatalf("llm call count = %d, want 2", len(state.calls))
	}
	if len(tools.executions) != 1 {
		t.Fatalf("tool executions = %d, want 1", len(tools.executions))
	}
	if tools.executions[0].name != "read_file" {
		t.Fatalf("executed tool = %q, want %q", tools.executions[0].name, "read_file")
	}

	secondCall := state.calls[1].messages
	if !hasAssistantToolCallMessage(secondCall, "call-1", "read_file") {
		t.Fatalf("second llm call missing assistant tool-call trace: %#v", secondCall)
	}
	if !hasToolResultMessage(secondCall, "call-1", "read_file", "README content") {
		t.Fatalf("second llm call missing tool result trace: %#v", secondCall)
	}

	session := repo.sessions[msg.SessionKey()]
	if session == nil {
		t.Fatalf("session %q not found", msg.SessionKey())
	}
	if len(session.Messages) != 4 {
		t.Fatalf("session message len = %d, want 4", len(session.Messages))
	}
	if session.Messages[1].Role != model.RoleAssistant || session.Messages[1].ToolCalls == nil {
		t.Fatalf("assistant tool-call trace not saved correctly: %#v", session.Messages[1])
	}
	if session.Messages[2].Role != model.RoleTool || session.Messages[2].ToolCallID != "call-1" {
		t.Fatalf("tool result trace not saved correctly: %#v", session.Messages[2])
	}
}

func TestService_ProcessMessage_ToolFailureIsRecorded(t *testing.T) {
	llm, state := newFakeLLMSequence(
		model.LLMResponse{
			ToolCalls: []*model.ToolCall{
				{
					ID:   "call-1",
					Name: "read_file",
					Args: map[string]any{"path": "missing.md"},
				},
			},
		},
		model.LLMResponse{Content: "Done."},
	)
	repo := newFakeSessionRepository()
	tools := newFakeToolRegistry()
	tools.err = errors.New("boom")

	service, err := NewService(repo, llm, tools, newTestPromptBuilder(t.TempDir()), 20, 8192, 0.7)
	if err != nil {
		t.Fatalf("NewService error: %v", err)
	}

	_, err = service.ProcessMessage(context.Background(), model.InboundMessage{
		ID:      "m-tool-failure",
		Channel: model.ChannelCLI,
		ChatID:  "local",
		Content: "read the missing file",
	})
	if err != nil {
		t.Fatalf("ProcessMessage error: %v", err)
	}

	if len(state.calls) != 2 {
		t.Fatalf("llm call count = %d, want 2", len(state.calls))
	}
	if !hasToolResultMessage(state.calls[1].messages, "call-1", "read_file", "Error: boom") {
		t.Fatalf("expected tool failure to be fed back into llm messages: %#v", state.calls[1].messages)
	}
}

func hasAssistantToolCallMessage(messages []map[string]any, callID string, toolName string) bool {
	for _, item := range messages {
		role, _ := item["role"].(string)
		if role != model.RoleAssistant {
			continue
		}

		toolCalls, ok := item["tool_calls"].([]map[string]any)
		if ok {
			for _, call := range toolCalls {
				if hasMatchingToolCall(call, callID, toolName) {
					return true
				}
			}
			continue
		}

		if rawCalls, ok := item["tool_calls"].([]any); ok {
			for _, candidate := range rawCalls {
				call, _ := candidate.(map[string]any)
				if hasMatchingToolCall(call, callID, toolName) {
					return true
				}
			}
		}
	}
	return false
}

func hasMatchingToolCall(call map[string]any, callID string, toolName string) bool {
	if call == nil {
		return false
	}
	id, _ := call["id"].(string)
	if id != callID {
		return false
	}
	function, _ := call["function"].(map[string]any)
	name, _ := function["name"].(string)
	return name == toolName
}

func hasToolResultMessage(messages []map[string]any, callID string, toolName string, content string) bool {
	for _, item := range messages {
		role, _ := item["role"].(string)
		if role != model.RoleTool {
			continue
		}
		toolCallID, _ := item["tool_call_id"].(string)
		name, _ := item["name"].(string)
		body, _ := item["content"].(string)
		if toolCallID == callID && name == toolName && body == content {
			return true
		}
	}
	return false
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

func TestToLLMMessages_AssistantToolCalls(t *testing.T) {
	now := model.NewSession("tmp").CreatedAt
	toolCalls := []map[string]any{
		{
			"id":   "call-1",
			"type": "function",
			"function": map[string]any{
				"name":      "read_file",
				"arguments": `{"path":"README.md"}`,
			},
		},
	}

	got := toLLMMessages([]*model.Message{
		{
			Role:      model.RoleAssistant,
			Content:   "Let me inspect that.",
			CreatedAt: now,
			ToolCalls: toolCalls,
		},
	})
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0]["role"] != model.RoleAssistant {
		t.Fatalf("role = %#v, want %q", got[0]["role"], model.RoleAssistant)
	}
	recovered, ok := got[0]["tool_calls"].([]map[string]any)
	if !ok {
		t.Fatalf("tool_calls type = %T, want []map[string]any", got[0]["tool_calls"])
	}
	if len(recovered) != 1 {
		t.Fatalf("tool_calls len = %d, want 1", len(recovered))
	}
	function, _ := recovered[0]["function"].(map[string]any)
	if function["name"] != "read_file" {
		t.Fatalf("function name = %#v, want %q", function["name"], "read_file")
	}
}
