package app

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	tooladapter "tinybot/internal/adapters/tool"
	"tinybot/internal/domain/model"
	chatservice "tinybot/internal/service/chat"
	transportbus "tinybot/internal/transport/bus"
	transportchannel "tinybot/internal/transport/channel"
	transportgateway "tinybot/internal/transport/gateway"
	transportruntime "tinybot/internal/transport/runtime"
)

type lockedBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return string(append([]byte(nil), b.data...))
}

type gatewaySessionRepo struct {
	mu       sync.Mutex
	sessions map[string]*model.Session
}

func (r *gatewaySessionRepo) GetOrCreateSession(ctx context.Context, key string) (*model.Session, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sessions == nil {
		r.sessions = make(map[string]*model.Session)
	}
	if session, ok := r.sessions[key]; ok {
		return session, nil
	}
	session := model.NewSession(key)
	r.sessions[key] = session
	return session, nil
}

func (r *gatewaySessionRepo) SaveSession(ctx context.Context, session *model.Session) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sessions == nil {
		r.sessions = make(map[string]*model.Session)
	}
	r.sessions[session.Key] = session
	return nil
}

func (r *gatewaySessionRepo) Session(key string) *model.Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.sessions[key]
}

type gatewayLLM struct {
	resp model.LLMResponse
}

func (g gatewayLLM) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error) {
	return g.resp, nil
}

type scriptedGatewayLLM struct {
	responses []model.LLMResponse
	callCount int
}

func (g *scriptedGatewayLLM) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error) {
	if g.callCount >= len(g.responses) {
		return model.LLMResponse{}, nil
	}
	resp := g.responses[g.callCount]
	g.callCount++
	return resp, nil
}

type gatewayTools struct{}

func (gatewayTools) GetDefinitions() []map[string]any {
	return nil
}

func (gatewayTools) Execute(ctx context.Context, name string, params map[string]any) (string, error) {
	return "", nil
}

type fakeRuntimeTrigger struct{}

func (fakeRuntimeTrigger) TriggerOnce(ctx context.Context) (string, error) {
	return "", nil
}

type failingGatewayProcessor struct {
	err error
}

func (f failingGatewayProcessor) ProcessMessage(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error) {
	return model.OutboundMessage{}, f.err
}

func TestWireMessageToolToBus_PublishesOutbound(t *testing.T) {
	reg := tooladapter.NewRegistry()
	reg.Register(tooladapter.NewMessageTool(nil, model.ChannelCLI, ""))
	reg.SetMessageContext(model.ChannelCLI, "gateway-chat")

	bus := transportbus.NewMemoryBus(1)
	defer bus.Close()

	wireMessageToolToBus(reg, bus)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := reg.Execute(ctx, "message", map[string]any{"content": "hello from tool"})
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	got, err := bus.ConsumeOutbound(ctx)
	if err != nil {
		t.Fatalf("ConsumeOutbound() error: %v", err)
	}
	if got.Channel != model.ChannelCLI {
		t.Fatalf("got.Channel = %q, want %q", got.Channel, model.ChannelCLI)
	}
	if got.ChatID != "gateway-chat" {
		t.Fatalf("got.ChatID = %q, want %q", got.ChatID, "gateway-chat")
	}
	if got.Content != "hello from tool" {
		t.Fatalf("got.Content = %q, want %q", got.Content, "hello from tool")
	}
}

func TestGatewayApp_Run_ConsoleRoundTrip(t *testing.T) {
	workspace := t.TempDir()
	repo := &gatewaySessionRepo{}
	llm := gatewayLLM{resp: model.LLMResponse{Content: "gateway reply"}}
	consolidator := chatservice.NewConsolidator(llm, 8192, 10)
	chatSvc, err := chatservice.NewService(
		repo,
		llm,
		gatewayTools{},
		newPromptBuilder(workspace),
		20,
		8192,
		0.7,
		consolidator,
	)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	b := transportbus.NewMemoryBus(16)
	loop := transportgateway.NewLoop(chatSvc, b)
	manager := transportchannel.NewChannelManager(b)
	output := &lockedBuffer{}
	manager.RegisterChannel(transportchannel.NewConsoleChannel(
		b,
		transportchannel.ConsoleChannelConfig{
			ChatID:     "gateway-chat",
			SenderID:   "gateway-user",
			Prompt:     "You>",
			ShowPrefix: true,
		},
		strings.NewReader("hello gateway\n"),
		output,
	))

	heartbeatRunner, err := transportruntime.NewHeartbeatRunner(fakeRuntimeTrigger{}, 60, false)
	if err != nil {
		t.Fatalf("NewHeartbeatRunner() error: %v", err)
	}
	cronRunner, err := transportruntime.NewCronRunner(fakeRuntimeTrigger{}, 60)
	if err != nil {
		t.Fatalf("NewCronRunner() error: %v", err)
	}

	gw := &GatewayApp{
		Bus:       b,
		Loop:      loop,
		Manager:   manager,
		Heartbeat: heartbeatRunner,
		Cron:      cronRunner,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer gw.Close()

	done := make(chan error, 1)
	go func() {
		done <- gw.Run(ctx)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if text := output.String(); strings.Contains(text, "tinybot> gateway reply") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	text := output.String()
	if !strings.Contains(text, "You> ") {
		t.Fatalf("gateway output missing prompt: %q", text)
	}
	if !strings.Contains(text, "tinybot> gateway reply") {
		t.Fatalf("gateway output missing assistant reply: %q", text)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("GatewayApp.Run() error: %v", err)
	}

	session := repo.Session("cli:gateway-chat")
	if session == nil {
		t.Fatal("expected gateway chat session to be saved")
	}
	if len(session.Messages) != 2 {
		t.Fatalf("session message len = %d, want 2", len(session.Messages))
	}
	if session.Messages[0].Content != "hello gateway" {
		t.Fatalf("user message = %q, want %q", session.Messages[0].Content, "hello gateway")
	}
	if session.Messages[1].Content != "gateway reply" {
		t.Fatalf("assistant message = %q, want %q", session.Messages[1].Content, "gateway reply")
	}
}

func TestGatewayApp_Run_MessageToolRoundTrip(t *testing.T) {
	workspace := t.TempDir()
	repo := &gatewaySessionRepo{}
	llm := &scriptedGatewayLLM{responses: []model.LLMResponse{
		{
			Content: "sending a tool message",
			ToolCalls: []*model.ToolCall{{
				ID:   "call-1",
				Name: "message",
				Args: map[string]any{"content": "tool hello"},
			}},
		},
		{Content: "final gateway reply"},
	}}

	reg := tooladapter.NewRegistry()
	reg.Register(tooladapter.NewMessageTool(nil, model.ChannelCLI, ""))

	consolidator := chatservice.NewConsolidator(llm, 8192, 10)
	chatSvc, err := chatservice.NewService(
		repo,
		llm,
		reg,
		newPromptBuilder(workspace),
		20,
		8192,
		0.7,
		consolidator,
	)
	if err != nil {
		t.Fatalf("NewService() error: %v", err)
	}

	b := transportbus.NewMemoryBus(16)
	wireMessageToolToBus(reg, b)
	loop := transportgateway.NewLoop(chatSvc, b)
	manager := transportchannel.NewChannelManager(b)
	output := &lockedBuffer{}
	manager.RegisterChannel(transportchannel.NewConsoleChannel(
		b,
		transportchannel.ConsoleChannelConfig{
			ChatID:     "gateway-chat",
			SenderID:   "gateway-user",
			Prompt:     "You>",
			ShowPrefix: true,
		},
		strings.NewReader("hello message tool\n"),
		output,
	))

	heartbeatRunner, err := transportruntime.NewHeartbeatRunner(fakeRuntimeTrigger{}, 60, false)
	if err != nil {
		t.Fatalf("NewHeartbeatRunner() error: %v", err)
	}
	cronRunner, err := transportruntime.NewCronRunner(fakeRuntimeTrigger{}, 60)
	if err != nil {
		t.Fatalf("NewCronRunner() error: %v", err)
	}

	gw := &GatewayApp{
		Bus:       b,
		Loop:      loop,
		Manager:   manager,
		Heartbeat: heartbeatRunner,
		Cron:      cronRunner,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer gw.Close()

	done := make(chan error, 1)
	go func() {
		done <- gw.Run(ctx)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		text := output.String()
		if strings.Contains(text, "tinybot> tool hello") && strings.Contains(text, "tinybot> final gateway reply") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	text := output.String()
	if !strings.Contains(text, "tinybot> tool hello") {
		t.Fatalf("gateway output missing message-tool reply: %q", text)
	}
	if !strings.Contains(text, "tinybot> final gateway reply") {
		t.Fatalf("gateway output missing final assistant reply: %q", text)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("GatewayApp.Run() error: %v", err)
	}

	session := repo.Session("cli:gateway-chat")
	if session == nil {
		t.Fatal("expected gateway chat session to be saved")
	}
	if len(session.Messages) != 4 {
		t.Fatalf("session message len = %d, want 4", len(session.Messages))
	}
	if session.Messages[0].Content != "hello message tool" {
		t.Fatalf("user message = %q, want %q", session.Messages[0].Content, "hello message tool")
	}
	if session.Messages[2].Content != "Message sent to cli:gateway-chat" {
		t.Fatalf("tool result = %q, want %q", session.Messages[2].Content, "Message sent to cli:gateway-chat")
	}
	if session.Messages[3].Content != "final gateway reply" {
		t.Fatalf("assistant message = %q, want %q", session.Messages[3].Content, "final gateway reply")
	}
}

func TestGatewayApp_Run_ConsoleFallbackOnProcessorError(t *testing.T) {
	b := transportbus.NewMemoryBus(16)
	loop := transportgateway.NewLoop(failingGatewayProcessor{err: context.DeadlineExceeded}, b)
	manager := transportchannel.NewChannelManager(b)
	output := &lockedBuffer{}
	manager.RegisterChannel(transportchannel.NewConsoleChannel(
		b,
		transportchannel.ConsoleChannelConfig{
			ChatID:     "gateway-chat",
			SenderID:   "gateway-user",
			Prompt:     "You>",
			ShowPrefix: true,
		},
		strings.NewReader("trigger failure\n"),
		output,
	))

	heartbeatRunner, err := transportruntime.NewHeartbeatRunner(fakeRuntimeTrigger{}, 60, false)
	if err != nil {
		t.Fatalf("NewHeartbeatRunner() error: %v", err)
	}
	cronRunner, err := transportruntime.NewCronRunner(fakeRuntimeTrigger{}, 60)
	if err != nil {
		t.Fatalf("NewCronRunner() error: %v", err)
	}

	gw := &GatewayApp{
		Bus:       b,
		Loop:      loop,
		Manager:   manager,
		Heartbeat: heartbeatRunner,
		Cron:      cronRunner,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer gw.Close()

	done := make(chan error, 1)
	go func() {
		done <- gw.Run(ctx)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if text := output.String(); strings.Contains(text, "Sorry, I encountered an error:") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	text := output.String()
	if !strings.Contains(text, "Sorry, I encountered an error: context deadline exceeded") {
		t.Fatalf("gateway output missing fallback reply: %q", text)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("GatewayApp.Run() error: %v", err)
	}
}

func TestBusResultDispatcher_DispatchPublishesOutbound(t *testing.T) {
	bus := transportbus.NewMemoryBus(1)
	defer bus.Close()

	dispatcher := &busResultDispatcher{bus: bus}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	want := model.OutboundMessage{
		Channel: model.ChannelTelegram,
		ChatID:  "chat-42",
		Content: "cron delivery",
	}

	if err := dispatcher.Dispatch(ctx, want); err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	got, err := bus.ConsumeOutbound(ctx)
	if err != nil {
		t.Fatalf("ConsumeOutbound() error: %v", err)
	}
	if got.Channel != want.Channel {
		t.Fatalf("got.Channel = %q, want %q", got.Channel, want.Channel)
	}
	if got.ChatID != want.ChatID {
		t.Fatalf("got.ChatID = %q, want %q", got.ChatID, want.ChatID)
	}
	if got.Content != want.Content {
		t.Fatalf("got.Content = %q, want %q", got.Content, want.Content)
	}
}
