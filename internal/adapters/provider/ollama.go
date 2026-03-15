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
	// defaultOllamaBaseURL 是 Ollama 的默认 API 地址
	// Ollama 在本地运行，默认端口 11434，提供 OpenAI 兼容的 API
	defaultOllamaBaseURL = "http://localhost:11434/v1"
	// defaultOllamaModel 是 Ollama 的默认模型
	// 用户需要在 Ollama 中先拉取对应的模型（如: ollama pull llama3.2）
	defaultOllamaModel = "llama3.2"
)

// OllamaProvider 实现了 CompletionClient 接口，用于与本地 Ollama 交互
//
// Ollama 是一个开源的大语言模型运行工具，支持在本地运行多种模型
// API 兼容 OpenAI 格式，因此可以复用 openai-go SDK
type OllamaProvider struct {
	client *openai.Client
	model  string
}

// NewOllamaProvider 创建一个新的 OllamaProvider
//
// 参数:
//
//	apiKey: Ollama 不需要 API Key，可以传空字符串
//
//	apiBase: Ollama 的 API 地址，默认 "http://localhost:11434/v1"
//
//	model: 使用的模型名称，默认 "llama3.2"
//
// 返回:
//
//	*OllamaProvider
//
//	error
func NewOllamaProvider(apiKey string, apiBase string, model string) (*OllamaProvider, error) {
	// Ollama 不需要 API Key，但为了接口一致性保留该参数
	// apiBase 为空时使用默认地址（本地 Ollama）
	if strings.TrimSpace(apiBase) == "" {
		apiBase = defaultOllamaBaseURL
	}
	if strings.TrimSpace(model) == "" {
		model = defaultOllamaModel
	}

	logger.Info("ollama provider initialized", "model", model, "api_base", apiBase)

	// 使用空的 API Key 创建 client（Ollama 不需要认证）
	client := openai.NewClient(
		option.WithAPIKey("ollama"), // Ollama 不需要真实的 API key
		option.WithBaseURL(apiBase),
	)

	return &OllamaProvider{
		client: &client,
		model:  model,
	}, nil
}

// Chat 发送聊天请求到 Ollama API
//
// 参数:
//
//	ctx: context.Context - 用于请求取消和超时控制
//
//	messages: 消息列表，每个消息包含 role 和 content
//
//	tools: 工具定义列表（可选）
//
//	maxTokens: 生成的最大 token 数（可选）
//
//	temperature: 温度参数，控制输出的随机性（可选）
//
// 返回:
//
//	model.LLMResponse - 包含生成的内容、工具调用和 token 使用情况
//
//	error
func (p *OllamaProvider) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error) {
	logger.Debug("ollama chat request", "model", p.model, "messages", len(messages))

	// 复用 qwen.go 中的转换函数
	convertedMessages, err := convertMessages(messages)
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("ollama provider convert messages: %w", err)
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
			return model.LLMResponse{}, fmt.Errorf("ollama provider convert tools: %w", err)
		}
		params.Tools = convertedTools
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("ollama provider chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return model.LLMResponse{}, errors.New("ollama provider: no choices in response")
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
