package provider

import (
	"context"
	"encoding/json"
	"strings"
	"tinybot/internal/domain/model"

	"github.com/openai/openai-go"
)

// streamChat 是所有 provider 共用的流式对话实现。
//
// 为什么提取成包内私有函数？
// - 4 个 provider 都使用同一个 openai-go SDK，流式逻辑完全一致
// - 按照 Rule of Three，第 3 次出现相同代码时应提取为共享函数
// - 保持私有，不暴露给包外，不增加 API 表面积
//
// 流程：
// 1. 复用 convertMessages/convertTools 构造请求参数
// 2. 调用 openai-go 的 NewStreaming 发起流式请求
// 3. 在 goroutine 中逐个消费 chunk，推送 Delta 事件给调用方
// 4. 累积 tool call 的 name/arguments 片段
// 5. 流结束时拼装完整 LLMResponse 作为 Done 事件
func streamChat(
	client *openai.Client,
	modelName string,
	ctx context.Context,
	messages []map[string]any,
	tools []map[string]any,
	maxTokens int,
	temperature float32,
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

		stream := client.Chat.Completions.NewStreaming(ctx, params)

		var contentBuilder strings.Builder
		type toolCallAccum struct {
			ID   string
			Name string
			Args strings.Builder
		}
		accum := make(map[int]*toolCallAccum)
		var finishReason string

		for stream.Next() {
			chunk := stream.Current()
			if len(chunk.Choices) == 0 {
				continue
			}
			choice := chunk.Choices[0]
			finishReason = choice.FinishReason

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

		resp := model.LLMResponse{
			Content:      strings.TrimSpace(contentBuilder.String()),
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
