package sessionrepo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tinybot/internal/domain/model"
)

func newTestSession(key string) *model.Session {
	s := model.NewSession(key)
	s.CreatedAt = time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)
	s.UpdatedAt = s.CreatedAt.Add(2 * time.Minute)
	s.Metadata["lang"] = "zh"
	s.LastConsolidated = 1
	s.AddMessage(model.RoleUser, "hello", nil)
	return s
}

func writeSessionFixture(t *testing.T, repo *FileSessionRepository, key string, content string) {
	t.Helper()

	path := repo.GetSessionPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error: %v", path, err)
	}
}

func TestFileSessionRepository_SaveSession(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		session *model.Session
		wantErr bool
		verify  func(t *testing.T, repo *FileSessionRepository, session *model.Session)
	}{
		{
			name:    "writes session and keeps round-trip data",
			ctx:     context.Background(),
			session: newTestSession("telegram:chat-1"),
			verify: func(t *testing.T, repo *FileSessionRepository, session *model.Session) {
				t.Helper()

				loaded, err := repo.LoadSession(session.Key)
				if err != nil {
					t.Fatalf("LoadSession(%q) error: %v", session.Key, err)
				}
				if loaded == nil {
					t.Fatal("loaded session is nil")
				}
				if loaded.Key != session.Key {
					t.Fatalf("loaded key = %q, want %q", loaded.Key, session.Key)
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
				if repo.cache[session.Key] != session {
					t.Fatalf("cache entry was not refreshed for %q", session.Key)
				}
			},
		},
		{
			name:    "rejects nil session",
			ctx:     context.Background(),
			session: nil,
			wantErr: true,
		},
		{
			name:    "honors canceled context before writing",
			ctx:     canceledCtx,
			session: newTestSession("telegram:chat-2"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewFileSessionRepository(t.TempDir())

			err := repo.SaveSession(tt.ctx, tt.session)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SaveSession() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.verify != nil {
				tt.verify(t, repo, tt.session)
			}
		})
	}
}

func TestFileSessionRepository_GetOrCreateSession(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		key     string
		setup   func(t *testing.T, repo *FileSessionRepository)
		wantErr bool
		verify  func(t *testing.T, repo *FileSessionRepository, got *model.Session)
	}{
		{
			name: "returns cached session",
			ctx:  context.Background(),
			key:  "cli:cached",
			setup: func(t *testing.T, repo *FileSessionRepository) {
				t.Helper()
				repo.cache["cli:cached"] = newTestSession("cli:cached")
			},
			verify: func(t *testing.T, repo *FileSessionRepository, got *model.Session) {
				t.Helper()
				if got == nil {
					t.Fatal("session is nil")
				}
				if got != repo.cache["cli:cached"] {
					t.Fatal("expected cached session instance")
				}
			},
		},
		{
			name: "loads session from disk",
			ctx:  context.Background(),
			key:  "cli:disk",
			setup: func(t *testing.T, repo *FileSessionRepository) {
				t.Helper()
				writeSessionFixture(t, repo, "cli:disk", `{"_type":"metadata","key":"cli:disk","metadata":{"lang":"zh"},"created_at":"2026-03-05T12:00:00Z","updated_at":"2026-03-05T12:02:00Z","last_consolidated":1}`+"\n"+`{"role":"user","content":"hello","created_at":"2026-03-05T12:01:00Z"}`+"\n")
			},
			verify: func(t *testing.T, repo *FileSessionRepository, got *model.Session) {
				t.Helper()
				if got == nil {
					t.Fatal("session is nil")
				}
				if got.Key != "cli:disk" {
					t.Fatalf("session key = %q, want %q", got.Key, "cli:disk")
				}
				if len(got.Messages) != 1 || got.Messages[0].Content != "hello" {
					t.Fatalf("loaded messages = %#v", got.Messages)
				}
				if repo.cache["cli:disk"] != got {
					t.Fatal("expected disk-loaded session to be cached")
				}
			},
		},
		{
			name: "creates session when file is missing",
			ctx:  context.Background(),
			key:  "cli:new",
			verify: func(t *testing.T, repo *FileSessionRepository, got *model.Session) {
				t.Helper()
				if got == nil {
					t.Fatal("session is nil")
				}
				if got.Key != "cli:new" {
					t.Fatalf("session key = %q, want %q", got.Key, "cli:new")
				}
				if len(got.Messages) != 0 {
					t.Fatalf("new session messages = %#v, want empty", got.Messages)
				}
			},
		},
		{
			name:    "returns error when context is canceled",
			ctx:     canceledCtx,
			key:     "cli:canceled",
			wantErr: true,
		},
		{
			name: "falls back to new session when stored file is invalid",
			ctx:  context.Background(),
			key:  "cli:broken",
			setup: func(t *testing.T, repo *FileSessionRepository) {
				t.Helper()
				writeSessionFixture(t, repo, "cli:broken", "{invalid json}\n")
			},
			verify: func(t *testing.T, repo *FileSessionRepository, got *model.Session) {
				t.Helper()
				if got == nil {
					t.Fatal("session is nil")
				}
				if got.Key != "cli:broken" {
					t.Fatalf("session key = %q, want %q", got.Key, "cli:broken")
				}
				if len(got.Messages) != 0 {
					t.Fatalf("fallback session messages = %#v, want empty", got.Messages)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewFileSessionRepository(t.TempDir())
			if tt.setup != nil {
				tt.setup(t, repo)
			}

			got, err := repo.GetOrCreateSession(tt.ctx, tt.key)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetOrCreateSession() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.verify != nil {
				tt.verify(t, repo, got)
			}
		})
	}
}

func TestFileSessionRepository_ListSessions_SortAndFallbackKey(t *testing.T) {
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

func TestFileSessionRepository_Invalidate(t *testing.T) {
	repo := NewFileSessionRepository(t.TempDir())
	session := newTestSession("cli:local")
	repo.cache["cli:local"] = session

	repo.Invalidate("cli:local")

	if _, ok := repo.cache["cli:local"]; ok {
		t.Fatal("expected cache entry to be removed")
	}
}
