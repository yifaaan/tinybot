package ports

import (
	"context"
	"tinybot/internal/domain/model"
)

type LLMClient interface {
	Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error)
}
