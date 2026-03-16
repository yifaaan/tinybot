package provider

import (
	"context"
	"encoding/json"
	"strings"
	"tinybot/internal/domain/model"
	"tinybot/internal/utils/logger"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// streamChat 是所有 provider 共用的流式对话实现。
//
// 流程：
// 1. 复用 convertMessages/convertTools 构造请求参数
// 2. 调用 openai-go 的 NewStreaming 发起流式请求
// 3. 在 goroutine 中逐个消费 chunk，推送 Delta 事件给调用方
// 4. 累积 tool call 的 name/arguments 片段
// 5. 流结束时拼装完整 LLMResponse 作为 Done 事件
// streamChat 是所有 provider 共用的流式对话实现。
//
// enableThinking 控制是否启用推理模式：
// - true: 注入 enable_thinking=true 参数，并从 delta 的 ExtraFields 中提取 reasoning_content
// - false: 标准流式行为，不做任何额外处理
//
// Qwen3 的流式 thinking 协议：
// 1. 模型先输出一连串 thinking delta（reasoning_content 字段有值，content 为空）
// 2. 然后输出正文 delta（content 字段有值，reasoning_content 为空）
// 3. 最后一个 chunk 的 finish_reason 标记流结束
func streamChat(
	client *openai.Client,
	modelName string,
	ctx context.Context,
	messages []map[string]any,
	tools []map[string]any,
	maxTokens int,
	temperature float32,
	enableThinking bool,
) <-chan model.StreamEvent {
	ch := make(chan model.StreamEvent)

	go func() {
		defer close(ch)

		convertedMessages, err := convertMessages(messages)
		if err != nil {
			ch <- model.StreamEvent{Kind: model.StreamEventError, Err: err}
			return
		}

		params := openai.ChatCompletionNewParams{
			Model:    modelName,
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
				ch <- model.StreamEvent{Kind: model.StreamEventError, Err: err}
				return
			}
			params.Tools = convertedTools
		}

		var reqOpts []option.RequestOption
		if enableThinking {
			reqOpts = append(reqOpts, option.WithJSONSet("enable_thinking", true))
		}

		stream := client.Chat.Completions.NewStreaming(ctx, params, reqOpts...)

		var contentBuilder strings.Builder
		var thinkingBuilder strings.Builder
		type toolCallAccum struct {
			ID   string
			Name string
			Args strings.Builder
		}
		accum := make(map[int]*toolCallAccum)
		var finishReason string

		chunkCount := 0
		for stream.Next() {
			chunk := stream.Current()
			if len(chunk.Choices) == 0 {
				continue
			}
			choice := chunk.Choices[0]
			finishReason = choice.FinishReason
			chunkCount++

			// 检查 thinking delta：厂商扩展字段 reasoning_content
			if enableThinking {
				if thinkingDelta := extractExtraString(choice.Delta.JSON.ExtraFields, "reasoning_content"); thinkingDelta != "" {
					thinkingBuilder.WriteString(thinkingDelta)
					ch <- model.StreamEvent{
						Kind:  model.StreamEventThinking,
						Delta: thinkingDelta,
					}
				}
			}

			if choice.Delta.Content != "" {
				contentBuilder.WriteString(choice.Delta.Content)
				ch <- model.StreamEvent{
					Kind:  model.StreamEventDelta,
					Delta: choice.Delta.Content,
				}
			}

			for _, tc := range choice.Delta.ToolCalls {
				idx := int(tc.Index)
				if _, ok := accum[idx]; !ok {
					accum[idx] = &toolCallAccum{ID: tc.ID}
				}
				a := accum[idx]
				if tc.Function.Name != "" {
					a.Name += tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					a.Args.WriteString(tc.Function.Arguments)
				}
			}
		}

		if err := stream.Err(); err != nil {
			ch <- model.StreamEvent{Kind: model.StreamEventError, Err: err}
			return
		}

		logger.Debug("stream finished",
			"total_chunks", chunkCount,
			"thinking_len", thinkingBuilder.Len(),
			"content_len", contentBuilder.Len(),
			"enableThinking", enableThinking,
		)

		resp := model.LLMResponse{
			Content:      strings.TrimSpace(contentBuilder.String()),
			Thinking:     strings.TrimSpace(thinkingBuilder.String()),
			FinishReason: finishReason,
		}

		if len(accum) > 0 {
			resp.ToolCalls = make([]*model.ToolCall, 0, len(accum))
			for _, a := range accum {
				args := make(map[string]any)
				raw := strings.TrimSpace(a.Args.String())
				if raw != "" {
					if err := json.Unmarshal([]byte(raw), &args); err != nil {
						args = map[string]any{"raw": raw}
					}
				}
				resp.ToolCalls = append(resp.ToolCalls, &model.ToolCall{
					ID:   a.ID,
					Name: a.Name,
					Args: args,
				})
			}
		}

		ch <- model.StreamEvent{Kind: model.StreamEventDone, Response: &resp}
	}()

	return ch
}
