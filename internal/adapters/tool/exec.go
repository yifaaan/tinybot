package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"tinybot/internal/utils/logger"
)

// ExecTool is tool to execute shell commands.
type ExecTool struct {
	timeout    int
	workingDir string
}

func NewExecTool(timeout int, workingDir string) *ExecTool {
	return &ExecTool{timeout: timeout, workingDir: workingDir}
}

func (t *ExecTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "exec",
		Description: "Execute a shell command and return its output. Use with caution.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"command":{
					"type":"string",
					"description":"The shell command to execute"
				},
				"working_dir":{
					"type":"string",
					"description":"Optional working directory for the command"
				}
			},
			"required":["command"]
		}`),
	}
}

func (t *ExecTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	command, _ := params["command"].(string)
	if strings.TrimSpace(command) == "" {
		return "", errors.New("exec tool: `command` is required")
	}

	workingDir, _ := params["working_dir"].(string)
	if workingDir == "" {
		workingDir = t.workingDir
	}

	timeout := time.Duration(t.timeout) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	logger.Info("exec tool running", "command", command)

	stdout, stderr, err := t.runCommand(ctx, command, workingDir)
	stderrText := strings.TrimSpace(stderr.String())

	var parts []string
	if stdout.Len() > 0 {
		parts = append(parts, stdout.String())
	}
	if stderr.Len() > 0 {
		parts = append(parts, "STDERR:\n"+stderr.String())
	}
	if err != nil {
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			return "", errors.New("exec tool: command timed out")
		case stderrText != "":
			return "", fmt.Errorf("exec tool: %w (stderr: %s)", err, stderrText)
		default:
			return "", fmt.Errorf("exec tool: %w", err)
		}
	}

	result := "(no output)"
	if len(parts) > 0 {
		result = strings.Join(parts, "\n")
	}

	const maxLen = 10000
	if len(result) > maxLen {
		result = result[:maxLen] + fmt.Sprintf("\n... (truncated, %d more chars)", len(result)-maxLen)
	}

	return result, nil
}

func (t *ExecTool) runCommand(ctx context.Context, command string, workingDir string) (bytes.Buffer, bytes.Buffer, error) {
	if runtime.GOOS == "windows" {
		return runWindowsCommand(ctx, command, workingDir)
	}
	return runExecCommand(ctx, workingDir, "sh", "-c", command)
}

func runWindowsCommand(ctx context.Context, command string, workingDir string) (bytes.Buffer, bytes.Buffer, error) {
	if !requiresWindowsShell(command) {
		args, err := splitWindowsCommandLine(command)
		if err == nil && len(args) > 0 {
			stdout, stderr, runErr := runExecCommand(ctx, workingDir, args[0], args[1:]...)
			if runErr == nil || !isCommandNotFound(runErr) {
				return stdout, stderr, runErr
			}
		}
	}
	return runExecCommand(ctx, workingDir, "cmd", "/c", command)
}

func runExecCommand(ctx context.Context, workingDir string, name string, args ...string) (bytes.Buffer, bytes.Buffer, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout, stderr, err
}

func requiresWindowsShell(command string) bool {
	inSingle := false
	inDouble := false

	for _, ch := range command {
		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '|', '>', '<', '&', ';', '(', ')':
			if !inSingle && !inDouble {
				return true
			}
		}
	}

	return false
}

func splitWindowsCommandLine(command string) ([]string, error) {
	var args []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	flush := func() {
		if current.Len() == 0 {
			return
		}
		args = append(args, current.String())
		current.Reset()
	}

	for i := 0; i < len(command); i++ {
		ch := command[i]
		switch ch {
		case '"':
			if inSingle {
				current.WriteByte(ch)
				continue
			}
			inDouble = !inDouble
		case '\'':
			if inDouble {
				current.WriteByte(ch)
				continue
			}
			inSingle = !inSingle
		case ' ', '\t':
			if inSingle || inDouble {
				current.WriteByte(ch)
				continue
			}
			flush()
		default:
			current.WriteByte(ch)
		}
	}

	if inSingle || inDouble {
		return nil, errors.New("unterminated quote in command")
	}

	flush()
	return args, nil
}

func isCommandNotFound(err error) bool {
	var execErr *exec.Error
	return errors.As(err, &execErr) && execErr.Err == exec.ErrNotFound
}
