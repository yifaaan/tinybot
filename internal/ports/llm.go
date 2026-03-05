package ports

import (
	"context"
	"tinybot/internal/domain/model"
)

type LLMClient interface {
	Chat(ctx context.Context, messages []map[string]any) (model.LLMResponse, error)
}
