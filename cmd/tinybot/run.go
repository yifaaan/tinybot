package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"
	"tinybot/internal/app"
)

// run is the testable entry point for the CLI.
//
// We keep main() tiny on purpose:
// - main() should only deal with process-level concerns such as os.Args and os.Stdout
// - run() contains command dispatch logic that we can unit test without spawning a real process
//
// workspace is injected as an argument so tests can use a temporary directory instead of touching
// the user's real .tinybot workspace.
func run(args []string, out io.Writer, workspace string) error {
	if out == nil {
		out = io.Discard
	}

	if len(args) == 0 {
		printHelp(out)
		return nil
	}

	cmd := strings.TrimSpace(args[0])
	switch cmd {
	case "help", "-h", "--help":
		printHelp(out)
		return nil
	case "onboard":
		return runOnboard(out, workspace)
	case "status":
		return runStatus(out, workspace)
	case "gateway":
		return runGateway(out, workspace)
	default:
		// Any non-command input is treated as a direct chat message.
		// This keeps the CLI ergonomic for the MVP: `tinybot 你好` still works.
		return runDirectChat(args, out, workspace)
	}
}

func runOnboard(out io.Writer, workspace string) error {
	result, err := app.OnBoard(context.Background(), workspace)
	if err != nil {
		return fmt.Errorf("onboard failed: %w", err)
	}

	_, _ = fmt.Fprintf(out, "Workspace ready at %s\n", result.Workspace)
	if len(result.CreatedFiles) == 0 {
		_, _ = fmt.Fprintln(out, "No new files created.")
		return nil
	}

	_, _ = fmt.Fprintln(out, "Created files:")
	for _, name := range result.CreatedFiles {
		_, _ = fmt.Fprintf(out, "  - %s\n", name)
	}

	return nil
}

func runStatus(out io.Writer, workspace string) error {
	status := app.CheckStatus(workspace)

	_, _ = fmt.Fprintf(out, "Workspace: %s\n", status.Workspace)
	_, _ = fmt.Fprintf(out, "Workspace exists: %t\n", status.WorkspaceExists)
	_, _ = fmt.Fprintf(out, "Config exists: %t\n", status.ConfigExists)
	_, _ = fmt.Fprintf(out, "Memory exists: %t\n", status.MemoryFileExists)
	_, _ = fmt.Fprintf(out, "Skills dir exists: %t\n", status.SkillsDirExists)
	_, _ = fmt.Fprintf(out, "Heartbeat file exists: %t\n", status.HeartbeatFileExists)

	if len(status.MissingFiles) > 0 {
		_, _ = fmt.Fprintln(out, "Missing files:")
		for _, name := range status.MissingFiles {
			_, _ = fmt.Fprintf(out, "  - %s\n", name)
		}
	}

	return nil
}

func runDirectChat(args []string, out io.Writer, workspace string) error {
	appInstance, err := app.NewApp(workspace)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}

	input := strings.TrimSpace(strings.Join(args, " "))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	reply, err := appInstance.ChatUseCase.ProcessDirect(ctx, "cli:direct", input)
	if err != nil {
		return fmt.Errorf("failed to process direct message: %w", err)
	}

	_, _ = fmt.Fprintln(out, reply)
	return nil
}

func runGateway(out io.Writer, workspace string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	gw, err := app.NewGatewayApp(workspace, os.Stdin, out)
	if err != nil {
		return fmt.Errorf("run gateway: %w", err)
	}
	defer gw.Close()

	_, _ = fmt.Fprintln(out, "tinybot gateway started (console channel mode)")
	_, _ = fmt.Fprintln(out, "Type a message and press Enter. Press Ctrl+C to stop.")
	return gw.Run(ctx)
}
