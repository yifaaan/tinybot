package heartbeat

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeAgent struct {
	calls []struct {
		sessionKey string
		content    string
	}
	reply string
	err   error
}

func (f *fakeAgent) ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error) {
	f.calls = append(f.calls, struct {
		sessionKey string
		content    string
	}{
		sessionKey: sessionKey,
		content:    content,
	})
	return f.reply, f.err
}

func TestTriggerOnce_SkipsWhenHeartbeatFileMissing(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	agent := &fakeAgent{}

	svc := NewService(workspace, agent, int(time.Minute), true)

	_, err := svc.TriggerOnce(context.Background())
	if err != nil {
		t.Fatalf("TriggerOnce() error: %v", err)
	}

	if len(agent.calls) != 0 {
		t.Fatalf("agent should not be called when HEARTBEAT.md is missing")
	}
}

func TestTriggerOnce_SkipsWhenHeartbeatFileHasNoActionableContent(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, "HEARTBEAT.md")

	content := `# Heartbeat

  <!-- comment -->

  - [ ]
  `
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	agent := &fakeAgent{}
	svc := NewService(workspace, agent, int(time.Minute), true)

	_, err := svc.TriggerOnce(context.Background())
	if err != nil {
		t.Fatalf("TriggerOnce() error: %v", err)
	}

	if len(agent.calls) != 0 {
		t.Fatalf("agent should not be called for non-actionable heartbeat content")
	}
}

func TestTriggerOnce_CallsAgentWhenHeartbeatFileIsActionable(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	path := filepath.Join(workspace, "HEARTBEAT.md")

	content := `# Heartbeat

  Review today's notes and remind me about unfinished tasks.
  `
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	agent := &fakeAgent{reply: "HEARTBEAT_OK"}
	svc := NewService(workspace, agent, int(time.Minute), true)

	reply, err := svc.TriggerOnce(context.Background())
	if err != nil {
		t.Fatalf("TriggerOnce() error: %v", err)
	}
	if reply != "HEARTBEAT_OK" {
		t.Fatalf("reply = %q, want %q", reply, "HEARTBEAT_OK")
	}
	if len(agent.calls) != 1 {
		t.Fatalf("len(agent.calls) = %d, want 1", len(agent.calls))
	}
	if agent.calls[0].sessionKey != "heartbeat" {
		t.Fatalf("sessionKey = %q, want %q", agent.calls[0].sessionKey, "heartbeat")
	}
}

func TestRun_ReturnsImmediatelyWhenDisabled(t *testing.T) {
	t.Parallel()

	svc := NewService(t.TempDir(), &fakeAgent{}, int(time.Minute), false)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := svc.Run(ctx); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}
