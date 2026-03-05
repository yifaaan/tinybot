package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"tinybot/internal/adapters/provider"
	"tinybot/internal/domain/model"
	"tinybot/internal/usecase/chat"

	"github.com/joho/godotenv"
)

const logo = `
  /\_/\
 ( o.o )  tinybot v0.1.0
  > ^ <   a tiny AI agent, rewritten in Go
`

func main() {
	fmt.Println(logo)

	_ = godotenv.Load()

	if len(os.Args) < 2 {
		fmt.Println("Usage: tinybot <your message>")
		return
	}

	workspace, err := os.Getwd()
	if err != nil {
		fmt.Println("failed to get workspace:", err)
		return
	}

	llm, err := provider.NewQwenClientFromEnv()
	if err != nil {
		fmt.Println("failed to init qwen client:", err)
		return
	}

	sessionRepo := model.NewFileSessionRepository(workspace)
	uc, err := chat.NewUseCase(sessionRepo, llm)
	if err != nil {
		fmt.Println("failed to init chat usecase:", err)
		return
	}

	input := strings.TrimSpace(strings.Join(os.Args[1:], " "))
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	reply, err := uc.ProcessDirect(ctx, "cli:direct", input)
	if err != nil {
		fmt.Println("failed to process direct message:", err)
		return
	}
	fmt.Println(reply)
}
