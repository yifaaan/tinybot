package model

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSession_AddMessage(t *testing.T) {
	tests := []struct {
		name       string
		initial    []*Message
		addRole    string
		addContent string
		attrs      map[string]any
		wantLen    int
		wantName   string
	}{
		{
			name:       "append to empty session",
			initial:    nil,
			addRole:    RoleUser,
			addContent: "hello",
			attrs:      nil,
			wantLen:    1,
		},
		{
			name: "append to existing session",
			initial: []*Message{
				{Role: RoleUser, Content: "hi", CreatedAt: time.Now()},
			},
			addRole:    RoleAssistant,
			addContent: "hello back",
			attrs:      nil,
			wantLen:    2,
		},
		{
			name:       "append with attrs",
			initial:    nil,
			addRole:    RoleTool,
			addContent: "result",
			attrs:      map[string]any{"tool_call_id": "call-1", "name": "search"},
			wantLen:    1,
			wantName:   "search",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{Key: "test", Messages: tt.initial}
			before := time.Now()
			s.AddMessage(tt.addRole, tt.addContent, tt.attrs)

			if len(s.Messages) != tt.wantLen {
				t.Errorf("got %d messages, want %d", len(s.Messages), tt.wantLen)
			}

			last := s.Messages[len(s.Messages)-1]
			if last.Role != tt.addRole {
				t.Errorf("got role %q, want %q", last.Role, tt.addRole)
			}
			if last.Content != tt.addContent {
				t.Errorf("got content %q, want %q", last.Content, tt.addContent)
			}
			if s.UpdatedAt.Before(before) {
				t.Error("UpdatedAt was not updated")
			}
			if tt.wantName != "" && last.Name != tt.wantName {
				t.Errorf("got name %q, want %q", last.Name, tt.wantName)
			}
		})
	}
}

func TestSession_GetHistory(t *testing.T) {
	now := time.Now()
	msgs := []*Message{
		{Role: RoleUser, Content: "a", CreatedAt: now},
		{Role: RoleAssistant, Content: "b", CreatedAt: now},
		{Role: RoleUser, Content: "c", CreatedAt: now},
		{Role: RoleAssistant, Content: "d", CreatedAt: now},
		{Role: RoleUser, Content: "e", CreatedAt: now},
	}

	tests := []struct {
		name             string
		messages         []*Message
		lastConsolidated int
		n                int
		wantLen          int
		wantFirst        string
	}{
		{
			name:             "get last 2 drops leading assistant",
			messages:         msgs,
			lastConsolidated: 0,
			n:                2,
			wantLen:          1, // last 2 = [assistant:d, user:e], drops leading "d"
			wantFirst:        "e",
		},
		{
			name:             "respects lastConsolidated",
			messages:         msgs,
			lastConsolidated: 3, // first 3 consolidated, unconsolidated = [d, e]
			n:                10,
			wantLen:          1, // drops leading assistant "d", only user "e" remains
			wantFirst:        "e",
		},
		{
			name:             "n=0 caps to 500 returns all unconsolidated",
			messages:         msgs,
			lastConsolidated: 0,
			n:                0,
			wantLen:          5,
			wantFirst:        "a",
		},
		{
			name: "drops leading non-user messages",
			messages: []*Message{
				{Role: RoleAssistant, Content: "orphan", CreatedAt: now},
				{Role: RoleTool, Content: "result", CreatedAt: now},
				{Role: RoleUser, Content: "hello", CreatedAt: now},
				{Role: RoleAssistant, Content: "hi", CreatedAt: now},
			},
			lastConsolidated: 0,
			n:                10,
			wantLen:          2,
			wantFirst:        "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{
				Key:              "test",
				Messages:         tt.messages,
				LastConsolidated: tt.lastConsolidated,
			}
			got := s.GetHistory(tt.n)
			if len(got) != tt.wantLen {
				t.Errorf("got %d messages, want %d", len(got), tt.wantLen)
				for i, m := range got {
					t.Logf("  [%d] role=%s content=%s", i, m.Role, m.Content)
				}
				return
			}
			if got[0].Content != tt.wantFirst {
				t.Errorf("first content = %q, want %q", got[0].Content, tt.wantFirst)
			}
		})
	}
}

func TestNewSession_Defaults(t *testing.T) {
	s := NewSession("telegram:42")
	if s.Key != "telegram:42" {
		t.Fatalf("key = %q, want %q", s.Key, "telegram:42")
	}
	if s.Messages == nil {
		t.Fatal("Messages should be initialized")
	}
	if s.Metadata == nil {
		t.Fatal("Metadata should be initialized")
	}
	if s.CreatedAt.IsZero() || s.UpdatedAt.IsZero() {
		t.Fatal("timestamps should be initialized")
	}
}

func TestSessionManager_SaveLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	m := NewSessionManager(tmp)

	s := NewSession("telegram:chat-1")
	s.CreatedAt = time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	s.UpdatedAt = s.CreatedAt.Add(2 * time.Minute)
	s.Metadata["lang"] = "zh"
	s.LastConsolidated = 1
	s.AddMessage(RoleUser, "hello", nil)

	if err := m.SaveSession(s); err != nil {
		t.Fatalf("saveSession error: %v", err)
	}

	loaded, err := m.LoadSession("telegram:chat-1")
	if err != nil {
		t.Fatalf("loadSession error: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded session is nil")
	}
	if loaded.Key != "telegram:chat-1" {
		t.Fatalf("loaded key = %q", loaded.Key)
	}
	if got, ok := loaded.Metadata["lang"].(string); !ok || got != "zh" {
		t.Fatalf("metadata lang = %#v, want %q", loaded.Metadata["lang"], "zh")
	}
	if loaded.LastConsolidated != 1 {
		t.Fatalf("LastConsolidated = %d, want 1", loaded.LastConsolidated)
	}
	if len(loaded.Messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(loaded.Messages))
	}
	if loaded.Messages[0].Role != RoleUser || loaded.Messages[0].Content != "hello" {
		t.Fatalf("loaded message = %#v", loaded.Messages[0])
	}
}

func TestSessionManager_ListSessions_SortAndFallbackKey(t *testing.T) {
	tmp := t.TempDir()
	m := NewSessionManager(tmp)

	// Newer metadata with explicit key.
	newerPath := filepath.Join(m.SessionDir, "telegram_100.jsonl")
	newer := `{"_type":"metadata","key":"telegram:100","created_at":"2026-03-05T00:00:00Z","updated_at":"2026-03-05T10:00:00Z","metadata":{},"last_consolidated":0}` + "\n"
	if err := os.WriteFile(newerPath, []byte(newer), 0o644); err != nil {
		t.Fatalf("write newer file: %v", err)
	}

	// Older metadata without key, fallback should use stem "cli_local" -> "cli:local".
	olderPath := filepath.Join(m.SessionDir, "cli_local.jsonl")
	older := `{"_type":"metadata","created_at":"2026-03-05T00:00:00Z","updated_at":"2026-03-05T09:00:00Z","metadata":{},"last_consolidated":0}` + "\n"
	if err := os.WriteFile(olderPath, []byte(older), 0o644); err != nil {
		t.Fatalf("write older file: %v", err)
	}

	got, err := m.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(ListSessions) = %d, want 2", len(got))
	}
	// Current behavior follows Python: descending by updated_at (newest first).
	if got[0]["key"] != "telegram:100" {
		t.Fatalf("first key = %#v, want %q", got[0]["key"], "telegram:100")
	}
	if got[1]["key"] != "cli:local" {
		t.Fatalf("fallback key = %#v, want %q", got[1]["key"], "cli:local")
	}
}

func TestSessionManager_GetOrCreateAndInvalidate(t *testing.T) {
	tmp := t.TempDir()
	m := NewSessionManager(tmp)

	s1 := m.GetOrCreateSession("cli:local")
	s1.AddMessage(RoleUser, "first", nil)
	if err := m.SaveSession(s1); err != nil {
		t.Fatalf("saveSession error: %v", err)
	}

	again := m.GetOrCreateSession("cli:local")
	if again != s1 {
		t.Fatal("expected cached session instance")
	}

	m.Invalidate("cli:local")
	reloaded := m.GetOrCreateSession("cli:local")
	if reloaded == s1 {
		t.Fatal("expected a reloaded instance after invalidate")
	}
	if len(reloaded.Messages) != 1 || reloaded.Messages[0].Content != "first" {
		t.Fatalf("reloaded messages = %#v", reloaded.Messages)
	}
}
