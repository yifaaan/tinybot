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

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// On Windows, use cmd.exe
		// Replace 'curl' with 'curl.exe' to avoid PowerShell alias
		command = strings.ReplaceAll(command, "curl ", "curl.exe ")
		// Remove unnecessary quotes around URLs (cmd.exe doesn't handle them well)
		command = strings.ReplaceAll(command, `curl.exe -s "`, "curl.exe -s ")
		command = strings.ReplaceAll(command, `curl.exe "`, "curl.exe ")
		command = strings.ReplaceAll(command, `" 2>&1`, " 2>&1")
		command = strings.TrimSuffix(command, `"`)
		cmd = exec.CommandContext(ctx, "cmd", "/c", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = workingDir

	logger.Info("exec tool running", "command", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
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
