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
	// defaultDeepseekBaseURL 是 Deepseek API 的默认地址
	defaultDeepseekBaseURL = "https://api.deepseek.com"
	// defaultDeepseekModel 是 Deepseek 的默认模型
	// deepseek-chat 是 Deepseek 的通用对话模型
	defaultDeepseekModel = "deepseek-chat"
)

// DeepseekProvider 实现了 CompletionClient 接口，用于与 Deepseek API 交互
//
// Deepseek 是一个提供高性能 LLM 服务的中国公司
// API 兼容 OpenAI 格式，因此可以复用 openai-go SDK
type DeepseekProvider struct {
	client *openai.Client
	model  string
}

// NewDeepseekProvider 创建一个新的 DeepseekProvider
//
// 参数:
//
//	apiKey: Deepseek API Key（必需）
//
//	apiBase: Deepseek 的 API 地址，默认 "https://api.deepseek.com"
//
//	model: 使用的模型名称，默认 "deepseek-chat"
//
// 返回:
//
//	*DeepseekProvider
//
//	error
func NewDeepseekProvider(apiKey string, apiBase string, model string) (*DeepseekProvider, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("deepseek provider: api key is required")
	}
	if strings.TrimSpace(apiBase) == "" {
		apiBase = defaultDeepseekBaseURL
	}
	if strings.TrimSpace(model) == "" {
		model = defaultDeepseekModel
	}

	logger.Info("deepseek provider initialized", "model", model)

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(apiBase),
	)

	return &DeepseekProvider{
		client: &client,
		model:  model,
	}, nil
}

// ChatStream 发起流式对话请求，委托给包内共享的 streamChat 实现。
func (p *DeepseekProvider) ChatStream(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) <-chan model.StreamEvent {
	return streamChat(p.client, p.model, ctx, messages, tools, maxTokens, temperature)
}

// Chat 发送聊天请求到 Deepseek API
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
func (p *DeepseekProvider) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error) {
	logger.Debug("deepseek chat request", "model", p.model, "messages", len(messages))

	// 复用 qwen.go 中的转换函数
	convertedMessages, err := convertMessages(messages)
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("deepseek provider convert messages: %w", err)
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
			return model.LLMResponse{}, fmt.Errorf("deepseek provider convert tools: %w", err)
		}
		params.Tools = convertedTools
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("deepseek provider chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return model.LLMResponse{}, errors.New("deepseek provider: no choices in response")
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
