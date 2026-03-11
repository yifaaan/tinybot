package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"tinybot/internal/app"
)

func runGateway(out io.Writer, workspace string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	gw, err := app.NewGatewayApp(workspace)
	if err != nil {
		return fmt.Errorf("run gateway: %w", err)
	}
	defer gw.Bus.Close()

	_, _ = fmt.Fprintln(out, "tinybot gateway started (local in-memory mode)")
	_, _ = fmt.Fprintln(out, "Press Ctrl+C to stop.")
	_, _ = fmt.Fprintln(out, "TODO: attach channels / cron producers and outbound consumers.")

	return gw.Run(ctx)
}
