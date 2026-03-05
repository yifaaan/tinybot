package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"tinybot/internal/domain/model"
)

func TestSessionRepository_SaveLoadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	m := NewFileSessionRepository(tmp)

	s := model.NewSession("telegram:chat-1")
	s.CreatedAt = time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	s.UpdatedAt = s.CreatedAt.Add(2 * time.Minute)
	s.Metadata["lang"] = "zh"
	s.LastConsolidated = 1
	s.AddMessage(model.RoleUser, "hello", nil)

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
	if loaded.Messages[0].Role != model.RoleUser || loaded.Messages[0].Content != "hello" {
		t.Fatalf("loaded message = %#v", loaded.Messages[0])
	}
}

func TestSessionRepository_ListSessions_SortAndFallbackKey(t *testing.T) {
	tmp := t.TempDir()
	m := NewFileSessionRepository(tmp)

	newerPath := filepath.Join(m.SessionDir, "telegram_100.jsonl")
	newer := `{"_type":"metadata","key":"telegram:100","created_at":"2026-03-05T00:00:00Z","updated_at":"2026-03-05T10:00:00Z","metadata":{},"last_consolidated":0}` + "\n"
	if err := os.WriteFile(newerPath, []byte(newer), 0o644); err != nil {
		t.Fatalf("write newer file: %v", err)
	}

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
	if got[0]["key"] != "telegram:100" {
		t.Fatalf("first key = %#v, want %q", got[0]["key"], "telegram:100")
	}
	if got[1]["key"] != "cli:local" {
		t.Fatalf("fallback key = %#v, want %q", got[1]["key"], "cli:local")
	}
}

func TestSessionRepository_GetOrCreateAndInvalidate(t *testing.T) {
	tmp := t.TempDir()
	m := NewFileSessionRepository(tmp)

	s1 := m.GetOrCreateSession("cli:local")
	s1.AddMessage(model.RoleUser, "first", nil)
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
