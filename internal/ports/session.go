package ports

import "tinybot/internal/domain/model"

type SessionManager interface {
	// Get the file path for a session.
	// Args:
	// 	key: Session key (usually channel:ChatID).
	// Returns:
	// 	The file path for the session.
	GetSessionPath(key string) string
	// Get an existing session or create a new one.
	GetOrCreateSession(key string) *model.Session
	// Load a session from the file system.
	LoadSession(key string) (*model.Session, error)
	// Save a session to the file system.
	SaveSession(s *model.Session) error
	// Remove a session from the in-memory cache.
	Invalidate(key string)
	// List all sessions.
	ListSessions() ([]map[string]any, error)
}
