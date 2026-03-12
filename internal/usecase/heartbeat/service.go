package heartbeat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const Prompt = `Read HEARTBEAT.md in your workspace (if it exists).
  Follow any instructions or tasks listed there.
  If nothing needs attention, reply with just: HEARTBEAT_OK`

const OKToken = "HEARTBEAT_OK"

var skipPatterns = map[string]bool{"- [ ]": true, "* [ ]": true, "- [x]": true, "* [x]": true}

type AgentTurner interface {
	ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error)
}

type Service struct {
	workspace       string
	agent           AgentTurner
	intervalSeconds time.Duration
	enabled         bool
}

func NewService(workspace string, agent AgentTurner, intervalSeconds int, enabled bool) *Service {
	return &Service{
		workspace:       workspace,
		agent:           agent,
		intervalSeconds: time.Duration(intervalSeconds) * time.Second,
		enabled:         enabled,
	}
}

func (s *Service) Run(ctx context.Context) error {
	if !s.enabled {
		return nil
	}

	ticker := time.NewTicker(s.intervalSeconds)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := s.TriggerOnce(ctx); err != nil {
				// TODO: log error
				continue
			}
		}
	}
}

func (s *Service) TriggerOnce(ctx context.Context) (string, error) {
	content, err := s.readHeartbeatFile()
	if err != nil {
		return "", err
	}
	if isHeartbeatEmpty(content) {
		return OKToken, nil
	}

	resp, err := s.agent.ProcessDirect(ctx, "heartbeat", Prompt)
	if err != nil {
		return "", err
	}
	return resp, nil
}

func (s *Service) readHeartbeatFile() (string, error) {
	path := filepath.Join(s.workspace, "HEARTBEAT.md")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No heartbeat file, treat as no actionable content
		}
		return "", err
	}
	return string(content), nil
}

// hasActionableContent determines if the content of HEARTBEAT.md requires action.
func isHeartbeatEmpty(content string) bool {
	if strings.TrimSpace(content) == "" {
		return true
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "<!--") || skipPatterns[line] {
			continue
		}
		return false
	}

	return true
}
