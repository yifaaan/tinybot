package repository

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
	"tinybot/internal/domain/model"
)

// FileSessionRepository manages conversation sessions.
// Sessions are stored as JSONL files in the sessions directory.
type FileSessionRepository struct {
	WorkSpace        string
	SessionDir       string
	LegacySessionDir string
	cache            map[string]*model.Session
}

func NewFileSessionRepository(workspace string) *FileSessionRepository {
	sessionDir := filepath.Join(workspace, "sessions")
	_ = os.MkdirAll(sessionDir, 0755)

	return &FileSessionRepository{
		WorkSpace:  workspace,
		SessionDir: sessionDir,
		cache:      make(map[string]*model.Session),
	}
}

// Get the file path for a session.
func (m *FileSessionRepository) GetSessionPath(key string) string {
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
func (m *FileSessionRepository) GetOrCreateSession(key string) *model.Session {
	if s, ok := m.cache[key]; ok {
		return s
	}

	s, _ := m.LoadSession(key)
	if s == nil {
		// slog.Info("creating new session", "key", key)
		s = model.NewSession(key)
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
func (m *FileSessionRepository) LoadSession(key string) (*model.Session, error) {
	path := m.GetSessionPath(key)
	_, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	messages := make([]*model.Message, 0)
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

		messages = append(messages, &model.Message{
			Role:       toString(data["role"]),
			Content:    toString(data["content"]),
			CreatedAt:  msgCreatedAt,
			ToolCalls:  data["tool_calls"],
			ToolCallID: toString(data["tool_call_id"]),
			Name:       toString(data["name"]),
		})
	}
	return &model.Session{
		Key:              key,
		Messages:         messages,
		CreatedAt:        sessionCreatedAt,
		UpdatedAt:        time.Now(),
		Metadata:         metadata,
		LastConsolidated: lastConsolidated,
	}, nil
}

func (m *FileSessionRepository) SaveSession(s *model.Session) error {
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
func (m *FileSessionRepository) Invalidate(key string) {
	delete(m.cache, key)
}

// List all sessions.
func (m *FileSessionRepository) ListSessions() ([]map[string]any, error) {
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
