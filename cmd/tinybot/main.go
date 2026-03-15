package main

import (
	"fmt"
	"io"
	"os"
	"tinybot/internal/app"
	"tinybot/internal/utils/logger"
)

const logo = `
  /\_/\
 ( o.o )  tinybot v0.1.0
  > ^ <   a tiny AI agent, rewritten in Go
`

func main() {
	logger.Init(logger.Config{
		Level:  "info",
		Format: "text",
		Output: "stderr",
	})
	fmt.Print(logo)

	workspace := app.DefaultWorkspacePath()
	if err := run(os.Args[1:], os.Stdout, workspace); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printHelp(out io.Writer) {
	_, _ = fmt.Fprintf(out, `Usage: tinybot <your message>
Commands:
  onboard - Initialize tinybot configuration and workspace
  gateway - Start tinybot in long-running local gateway mode
  status - Show current status of tinybot
  help - Show this help message
`)
}
