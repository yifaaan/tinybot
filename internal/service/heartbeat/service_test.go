package heartbeat

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type fakeHeartbeatAgent struct {
	calls []struct {
		sessionKey string
		content    string
	}
	reply string
	err   error
}

func (f *fakeHeartbeatAgent) ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error) {
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
	agent := &fakeHeartbeatAgent{}
	svc := NewService(workspace, agent)

	reply, err := svc.TriggerOnce(context.Background())
	if err != nil {
		t.Fatalf("TriggerOnce() error: %v", err)
	}
	if reply != OKToken {
		t.Fatalf("reply = %q, want %q", reply, OKToken)
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

	agent := &fakeHeartbeatAgent{}
	svc := NewService(workspace, agent)

	reply, err := svc.TriggerOnce(context.Background())
	if err != nil {
		t.Fatalf("TriggerOnce() error: %v", err)
	}
	if reply != OKToken {
		t.Fatalf("reply = %q, want %q", reply, OKToken)
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

	agent := &fakeHeartbeatAgent{reply: "HEARTBEAT_OK"}
	svc := NewService(workspace, agent)

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

func TestIsHeartbeatEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "empty", content: "", want: true},
		{name: "headers only", content: "# Heartbeat\n\n<!-- note -->\n\n- [ ]", want: true},
		{name: "actionable", content: "# Heartbeat\n\nReview the backlog", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsHeartbeatEmpty(tt.content); got != tt.want {
				t.Fatalf("IsHeartbeatEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}
