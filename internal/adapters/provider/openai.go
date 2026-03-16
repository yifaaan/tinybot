package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"tinybot/internal/domain/model"
	"tinybot/internal/utils/logger"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	defaultOpenAIModel   = "gpt-4o-mini"
)

type OpenAIProvider struct {
	client *openai.Client
	model  string
}

func NewOpenAIProvider(apiKey string, apiBase string, model string) (*OpenAIProvider, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("openai provider: api key is required")
	}
	if strings.TrimSpace(apiBase) == "" {
		apiBase = defaultOpenAIBaseURL
	}
	if strings.TrimSpace(model) == "" {
		model = defaultOpenAIModel
	}

	logger.Info("openai provider initialized", "model", model)

	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(apiBase))

	return &OpenAIProvider{
		client: &client,
		model:  model,
	}, nil
}

// ChatStream 发起流式对话请求，委托给包内共享的 streamChat 实现。
func (p *OpenAIProvider) ChatStream(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) <-chan model.StreamEvent {
	return streamChat(p.client, p.model, ctx, messages, tools, maxTokens, temperature, false)
}

// Chat 发送聊天请求到 OpenAI API
func (p *OpenAIProvider) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error) {
	logger.Debug("openai chat request", "model", p.model, "messages", len(messages))

	convertedMessages, err := convertMessages(messages)
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("openai provider convert messages: %w", err)
	}

	params := openai.ChatCompletionNewParams{
		Model:    p.model,
		Messages: convertedMessages,
	}
	if maxTokens > 0 {
		params.MaxTokens = openai.Int(int64(maxTokens))
	}
	if temperature >= 0 {
		params.Temperature = openai.Float(float64(temperature))
	}
	if len(tools) > 0 {
		convertedTools, err := convertTools(tools)
		if err != nil {
			return model.LLMResponse{}, fmt.Errorf("openai provider convert tools: %w", err)
		}
		params.Tools = convertedTools
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("openai provider chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return model.LLMResponse{}, errors.New("openai provider: no choices in response")
	}

	choice := resp.Choices[0]
	out := model.LLMResponse{
		Content:      strings.TrimSpace(choice.Message.Content),
		FinishReason: choice.FinishReason,
		PromptTokens: int(resp.Usage.PromptTokens),
		OutputTokens: int(resp.Usage.CompletionTokens),
	}

	// 解析工具调用
	if len(choice.Message.ToolCalls) > 0 {
		out.ToolCalls = make([]*model.ToolCall, 0, len(choice.Message.ToolCalls))
		for _, tc := range choice.Message.ToolCalls {
			args := make(map[string]any)
			rawArgs := strings.TrimSpace(tc.Function.Arguments)
			if rawArgs != "" {
				if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
					args = map[string]any{"raw": rawArgs}
				}
			}
			out.ToolCalls = append(out.ToolCalls, &model.ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: args,
			})
		}
	}

	return out, nil
}
