package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"tinybot/internal/app"
)

const logo = `
  /\_/\
 ( o.o )  tinybot v0.1.0
  > ^ <   a tiny AI agent, rewritten in Go
`

func main() {
	fmt.Print(logo)

	if len(os.Args) < 2 {
		fmt.Println("Usage: tinybot <your message>")
		return
	}

	workspace, err := os.Getwd()
	app, err := app.NewApp(workspace)
	if err != nil {
		fmt.Println("failed to create app:", err)
		return
	}

	input := strings.TrimSpace(strings.Join(os.Args[1:], " "))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	reply, err := app.ChatUseCase.ProcessDirect(ctx, "cli:direct", input)
	if err != nil {
		fmt.Println("failed to process direct message:", err)
		return
	}
	fmt.Println(reply)
}
