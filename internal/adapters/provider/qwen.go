package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"tinybot/internal/domain/model"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	defaultQwenBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	defaultQwenModel   = "qwen3-max"
)

type QwenProvider struct {
	client *openai.Client
	model  string
}

// NewQwenProvider creates a new QwenProvider.
// Args:
//
//	apiKey: string
//
//	apiBase: string
//
//	model: string
//
// Returns:
//
//	*QwenProvider
//
//	error
func NewQwenProvider(apiKey string, apiBase string, model string) (*QwenProvider, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("qwen provider: api key is required")
	}
	if strings.TrimSpace(apiBase) == "" {
		apiBase = defaultQwenBaseURL
	}
	if strings.TrimSpace(model) == "" {
		model = defaultQwenModel
	}
	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(apiBase))
	return &QwenProvider{
		client: &client,
		model:  model,
	}, nil
}

func NewQwenClientFromEnv() (*QwenProvider, error) {
	apiKey := os.Getenv("DASHSCOPE_API_KEY")
	apiBase := os.Getenv("DASHSCOPE_BASE_URL")
	model := os.Getenv("QWEN_MODEL")
	return NewQwenProvider(apiKey, apiBase, model)
}

// Chat sends a chat completion request to the Qwen model.
// Args:
//
//	ctx: context.Context
//
//	messages: []map[string]any (List of message maps with 'role' and 'content')
//
// Returns:
//
//	model.LLMResponse
//
//	error
func (q *QwenProvider) Chat(ctx context.Context, messages []map[string]any) (model.LLMResponse, error) {
	converted := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))

	for _, m := range messages {
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)

		// TODO: Handle tool calls and tool call IDs

		switch role {
		case "system":
			converted = append(converted, openai.SystemMessage(content))
		case "assistant":
			converted = append(converted, openai.AssistantMessage(content))
		// TODO: Handle tool calls and tool call IDs
		default:
			converted = append(converted, openai.UserMessage(content))
		}
	}

	resp, err := q.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    q.model,
		Messages: converted,
	})
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("qwen provider chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return model.LLMResponse{}, errors.New("qwen provider chat: no choices returned")
	}

	choice := resp.Choices[0]
	return model.LLMResponse{
		Content:      strings.TrimSpace(choice.Message.Content),
		FinishReason: choice.FinishReason,
	}, nil
}
