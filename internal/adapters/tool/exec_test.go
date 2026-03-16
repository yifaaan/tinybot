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
		if !strings.Contains(err.Error(), "command timed out") {
			t.Fatalf("unexpected timeout error: %v", err)
		}
	})
}

func TestRequiresWindowsShell(t *testing.T) {
	if requiresWindowsShell(`curl -s "https://wttr.in/Xian?format=%l:+%c+%t"`) {
		t.Fatal("simple curl command should not require shell")
	}
	if !requiresWindowsShell(`curl -s https://wttr.in/Xian?0 2>&1`) {
		t.Fatal("redirection should require shell")
	}
	if !requiresWindowsShell(`curl -s https://wttr.in/Xian?0 | findstr Xian`) {
		t.Fatal("pipe should require shell")
	}
}

func TestSplitWindowsCommandLine_ParsesQuotedURL(t *testing.T) {
	args, err := splitWindowsCommandLine(`curl -s --connect-timeout 5 --max-time 10 "https://wttr.in/Xian?format=%l:+%c+%t"`)
	if err != nil {
		t.Fatalf("splitWindowsCommandLine() error = %v", err)
	}

	if len(args) != 7 {
		t.Fatalf("arg count = %d, want 7, args=%q", len(args), args)
	}
	if got := args[0]; got != "curl" {
		t.Fatalf("argv[0] = %q, want curl", got)
	}
	if got := args[6]; got != "https://wttr.in/Xian?format=%l:+%c+%t" {
		t.Fatalf("argv[6] = %q", got)
	}
}

func TestSplitWindowsCommandLine_UnterminatedQuote(t *testing.T) {
	_, err := splitWindowsCommandLine(`curl -s "https://wttr.in/Xian?format=3`)
	if err == nil {
		t.Fatal("expected unterminated quote error")
	}
}

func TestIsCommandNotFound(t *testing.T) {
	_, _, err := runExecCommand(context.Background(), "", "command-that-does-not-exist-tinybot-test")
	if err == nil {
		t.Fatal("expected command-not-found error")
	}
	if !isCommandNotFound(err) {
		t.Fatalf("expected command-not-found classification, got %T %v", err, err)
	}
}
