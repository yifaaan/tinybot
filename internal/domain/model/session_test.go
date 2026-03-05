package model

import (
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

