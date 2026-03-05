package model

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Role constants for chat messages.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

type Message struct {
	Role       string    `json:"role"` // user, assistant, tool, system
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	ToolCalls  any       `json:"tool_calls,omitempty"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
	Name       string    `json:"name,omitempty"`
}

type Session struct {
	Key              string         `json:"key"` // channel:ChatID
	Messages         []*Message     `json:"-"`   // stored in JSONL entries, not in metadata line
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	Metadata         map[string]any `json:"metadata"`
	LastConsolidated int            `json:"last_consolidated"` // Number of messages already consolidated
}

func NewSession(key string) *Session {
	return &Session{
		Key:       key,
		Messages:  make([]*Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// AddMessage adds a new message and updates the timestamp.
func (s *Session) AddMessage(role, content string, attrs map[string]any) {
	msg := &Message{
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}

	if attrs != nil {
		if v, ok := attrs["tool_calls"]; ok {
			msg.ToolCalls = v
		}
		if v, ok := attrs["tool_call_id"]; ok {
			if id, ok := v.(string); ok {
				msg.ToolCallID = id
			}
		}
		if v, ok := attrs["name"]; ok {
			if name, ok := v.(string); ok {
				msg.Name = name
			}
		}
	}

	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// GetHistory returns unconsolidated history capped at n (default 500),
// and trims leading non-user messages to avoid orphan tool chains.
func (s *Session) GetHistory(n int) []*Message {
	if n <= 0 {
		n = 500
	}

	start := s.LastConsolidated
	if start < 0 || start > len(s.Messages) {
		start = 0
	}
	unconsolidated := s.Messages[start:]
	if len(unconsolidated) > n {
		unconsolidated = unconsolidated[len(unconsolidated)-n:]
	}

	sliced := unconsolidated
	// Drop leading non-user messages to avoid orphaned tool_result blocks
	for i, m := range sliced {
		if m.Role == RoleUser {
			sliced = sliced[i:]
			break
		}
	}
	return sliced
}

// FileSessionManager manages conversation sessions.
// Sessions are stored as JSONL files in the sessions directory.
type FileSessionManager struct {
	WorkSpace        string
	SessionDir       string
	LegacySessionDir string
	cache            map[string]*Session
}

func NewSessionManager(workspace string) *FileSessionManager {
	sessionDir := filepath.Join(workspace, "sessions")
	_ = os.MkdirAll(sessionDir, 0755)

	return &FileSessionManager{
		WorkSpace:  workspace,
		SessionDir: sessionDir,
		cache:      make(map[string]*Session),
	}
}

// Get the file path for a session.
func (m *FileSessionManager) GetSessionPath(key string) string {
	safeKey := safeFilename(strings.ReplaceAll(key, ":", "_"))
	return filepath.Join(m.SessionDir, fmt.Sprintf("%s.jsonl", safeKey))
}

// Replace unsafe path characters with underscores.
func safeFilename(name string) string {
	return regexp.MustCompile(`[<>:"/\\|?*]`).ReplaceAllString(name, "_")
}

// GetOrCreateSession gets an existing session or creates a new one.
// Args:
//
//	key: Session key (usually channel:ChatID).
//
// Returns:
//
//	The session.
func (m *FileSessionManager) GetOrCreateSession(key string) *Session {
	if s, ok := m.cache[key]; ok {
		return s
	}

	s, _ := m.LoadSession(key)
	if s == nil {
		s = NewSession(key)
	}
	m.cache[key] = s
	return s
}

// Session JSONL file structure:
//
//	line 1: metadata object
//	  {
//	    "_type": "metadata",
//	    "key": "<session_key>",
//	    "metadata": { ... },
//	    "created_at": "RFC3339 time string",
//	    "updated_at": "RFC3339 time string",
//	    "last_consolidated": <int>
//	  }
//	line 2..N: message objects (one message per line)
//	  {
//	    "role": "user|assistant|tool|system",
//	    "content": "...",
//	    "created_at": "RFC3339 time string",
//	    "tool_calls": ...,
//	    "tool_call_id": "...",
//	    "name": "..."
//	  }
func (m *FileSessionManager) LoadSession(key string) (*Session, error) {
	path := m.GetSessionPath(key)
	_, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	messages := make([]*Message, 0)
	metadata := make(map[string]any)
	sessionCreatedAt := time.Now()
	lastConsolidated := 0

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		data := make(map[string]any)
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			return nil, err
		}
		if typ, _ := data["_type"].(string); typ == "metadata" {
			if md, ok := data["metadata"].(map[string]any); ok {
				metadata = md
			}
			if t, ok := parseJSONTime(data["created_at"]); ok {
				sessionCreatedAt = t
			}
			lastConsolidated = toInt(data["last_consolidated"])
			continue
		}

		msgCreatedAt := time.Now()
		if t, ok := parseJSONTime(data["created_at"]); ok {
			msgCreatedAt = t
		} else if t, ok := parseJSONTime(data["timestamp"]); ok {
			msgCreatedAt = t
		}

		messages = append(messages, &Message{
			Role:       toString(data["role"]),
			Content:    toString(data["content"]),
			CreatedAt:  msgCreatedAt,
			ToolCalls:  data["tool_calls"],
			ToolCallID: toString(data["tool_call_id"]),
			Name:       toString(data["name"]),
		})
	}
	return &Session{
		Key:              key,
		Messages:         messages,
		CreatedAt:        sessionCreatedAt,
		UpdatedAt:        time.Now(),
		Metadata:         metadata,
		LastConsolidated: lastConsolidated,
	}, nil
}

func (m *FileSessionManager) SaveSession(s *Session) error {
	if s == nil {
		return errors.New("session is nil")
	}
	path := m.GetSessionPath(s.Key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("save session mkdir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)

	metadataLine := map[string]any{
		"_type":             "metadata",
		"key":               s.Key,
		"metadata":          s.Metadata,
		"created_at":        s.CreatedAt.Format(time.RFC3339),
		"updated_at":        s.UpdatedAt.Format(time.RFC3339),
		"last_consolidated": s.LastConsolidated,
	}
	jsonData, err := json.Marshal(metadataLine)
	if err != nil {
		return err
	}
	writer.Write(jsonData)
	writer.WriteString("\n")
	for _, msg := range s.Messages {
		jsonData, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		writer.Write(jsonData)
		writer.WriteString("\n")
	}
	writer.Flush()
	m.cache[s.Key] = s
	return nil
}

// Remove a session from the in-memory cache.
func (m *FileSessionManager) Invalidate(key string) {
	delete(m.cache, key)
}

// List all sessions.
func (m *FileSessionManager) ListSessions() ([]map[string]any, error) {
	filenames, err := filepath.Glob(filepath.Join(m.SessionDir, "*.jsonl"))
	if err != nil {
		return nil, err
	}

	sessions := make([]map[string]any, 0)
	for _, filename := range filenames {
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}

		reader := bufio.NewReader(f)
		firstLine, err := reader.ReadString('\n')
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		_ = f.Close()

		firstLine = strings.TrimSpace(firstLine)
		if firstLine != "" {
			data := make(map[string]any)
			if err := json.Unmarshal([]byte(firstLine), &data); err != nil {
				return nil, err
			}
			if typ, _ := data["_type"].(string); typ == "metadata" {
				key, ok := data["key"].(string)
				if !ok {
					stem := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
					key = strings.Replace(stem, "_", ":", 1)
				}
				sessions = append(sessions, map[string]any{
					"key":        key,
					"created_at": data["created_at"],
					"updated_at": data["updated_at"],
					"path":       filename,
				})
			}
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		ti, errI := time.Parse(time.RFC3339, fmt.Sprint(sessions[i]["updated_at"]))
		tj, errJ := time.Parse(time.RFC3339, fmt.Sprint(sessions[j]["updated_at"]))
		if errI != nil || errJ != nil {
			return fmt.Sprint(sessions[i]["updated_at"]) < fmt.Sprint(sessions[j]["updated_at"])
		}
		return ti.After(tj)
	})
	return sessions, nil
}

func toString(v any) string {
	s, _ := v.(string)
	return s
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func parseJSONTime(v any) (time.Time, bool) {
	s, ok := v.(string)
	if !ok || s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
