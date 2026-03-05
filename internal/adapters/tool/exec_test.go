package tool

import (
	"context"
	"runtime"
	"strings"
	"testing"
)

func TestExecTool_Execute(t *testing.T) {
	t.Run("missing command", func(t *testing.T) {
		tool := NewExecTool(2, "")
		_, err := tool.Execute(context.Background(), map[string]any{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("success output", func(t *testing.T) {
		tool := NewExecTool(5, "")
		cmd := "echo hello"
		out, err := tool.Execute(context.Background(), map[string]any{"command": cmd})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(strings.ToLower(out), "hello") {
			t.Fatalf("unexpected output: %q", out)
		}
		// t.Fatalf("iam:\n\n%s", out)
	})

	t.Run("command failed", func(t *testing.T) {
		tool := NewExecTool(5, "")
		cmd := "not a real command"
		_, err := tool.Execute(context.Background(), map[string]any{"command": cmd})
		if err == nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(err.Error(), "exec tool") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		tool := NewExecTool(1, "")
		cmd := "sleep 3"
		if runtime.GOOS == "windows" {
			cmd = "ping 127.0.0.1 -n 4 > nul"
		}
		_, err := tool.Execute(context.Background(), map[string]any{"command": cmd})
		if err == nil {
			t.Fatalf("exepeted timeout error, got nil")
		}
	})
}
