package sessionrepo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"tinybot/internal/domain/model"
)

func (m *FileSessionRepository) DeleteSession(key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("delete session: key is required")
	}

	path := m.GetSessionPath(key)
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete session: %w", err)
	}
	delete(m.cache, key)
	return nil
}

func (m *FileSessionRepository) RenameSession(ctx context.Context, key string, title string) (*model.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("rename session: %w", err)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("rename session: key is required")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("rename session: title is required")
	}

	session, err := m.LoadSession(key)
	if err != nil {
		return nil, fmt.Errorf("rename session: %w", err)
	}
	if session.Metadata == nil {
		session.Metadata = make(map[string]any)
	}
	session.Metadata["title"] = title
	session.UpdatedAt = time.Now()
	if err := m.SaveSession(ctx, session); err != nil {
		return nil, fmt.Errorf("rename session save: %w", err)
	}
	return session, nil
}
