package heartbeat

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const Prompt = `Read HEARTBEAT.md in your workspace (if it exists).
  Follow any instructions or tasks listed there.
  If nothing needs attention, reply with just: HEARTBEAT_OK`

const OKToken = "HEARTBEAT_OK"

var skipPatterns = map[string]bool{"- [ ]": true, "* [ ]": true, "- [x]": true, "* [x]": true}

// Service evaluates the heartbeat file once and optionally triggers a direct chat turn.
//
// Responsibilities:
//   - read HEARTBEAT.md from the workspace
//   - decide whether it contains actionable content
//   - invoke the chat service only when action is required
//
// Inputs: a workspace path and the current execution context.
// Outputs: either OKToken when no action is needed, or the direct-turn reply text.
// State changes: none inside the service itself.
// Side effects: reads HEARTBEAT.md from disk and may call the chat service.
// Compatibility: preserves the current Go heartbeat behavior while separating periodic scheduling into a runtime layer.
type Service struct {
	workspace string
	agent     AgentTurner
}

// NewService constructs a heartbeat service bound to one workspace.
func NewService(workspace string, agent AgentTurner) *Service {
	return &Service{
		workspace: workspace,
		agent:     agent,
	}
}

// TriggerOnce evaluates HEARTBEAT.md one time.
func (s *Service) TriggerOnce(ctx context.Context) (string, error) {
	content, err := s.readHeartbeatFile()
	if err != nil {
		return "", err
	}
	if IsHeartbeatEmpty(content) {
		return OKToken, nil
	}
	if s.agent == nil {
		return "", errors.New("heartbeat service: agent is required")
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
			return "", nil
		}
		return "", err
	}
	return string(content), nil
}

// IsHeartbeatEmpty reports whether HEARTBEAT.md contains actionable content.
func IsHeartbeatEmpty(content string) bool {
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
