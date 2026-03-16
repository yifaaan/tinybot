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
	"github.com/openai/openai-go/packages/respjson"
	"github.com/openai/openai-go/shared"
)

const (
	defaultQwenBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	defaultQwenModel   = "qwen3-max"
)

type QwenProvider struct {
	client         *openai.Client
	model          string
	enableThinking bool
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
func NewQwenProvider(apiKey string, apiBase string, model string, enableThinking bool) (*QwenProvider, error) {
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
		client:         &client,
		model:          model,
		enableThinking: enableThinking,
	}, nil
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
func (q *QwenProvider) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error) {
	logger.Debug("llm chat request", "model", q.model, "messages", len(messages), "tools", len(tools), "thinking", q.enableThinking)

	convertedMessages, err := convertMessages(messages)
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("qwen provider convert messages: %w", err)
	}

	params := openai.ChatCompletionNewParams{
		Model:    q.model,
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
			return model.LLMResponse{}, fmt.Errorf("qwen provider convert tools: %w", err)
		}
		params.Tools = convertedTools
	}

	// 构建请求选项：如果启用 thinking，通过 WithJSONSet 注入厂商扩展参数
	var reqOpts []option.RequestOption
	if q.enableThinking {
		reqOpts = append(reqOpts, option.WithJSONSet("enable_thinking", true))
	}

	resp, err := q.client.Chat.Completions.New(ctx, params, reqOpts...)
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("qwen provider chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return model.LLMResponse{}, errors.New("qwen provider chat: no choices returned")
	}

	choice := resp.Choices[0]
	out := model.LLMResponse{
		Content:      strings.TrimSpace(choice.Message.Content),
		Thinking:     extractExtraString(choice.Message.JSON.ExtraFields, "reasoning_content"),
		FinishReason: choice.FinishReason,
		PromptTokens: int(resp.Usage.PromptTokens),
		OutputTokens: int(resp.Usage.CompletionTokens),
	}

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

func (q *QwenProvider) ChatStream(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) <-chan model.StreamEvent {
	return streamChat(q.client, q.model, ctx, messages, tools, maxTokens, temperature, q.enableThinking)
}

func convertMessages(messages []map[string]any) ([]openai.ChatCompletionMessageParamUnion, error) {
	converted := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))

	for _, message := range messages {
		role := stringValue(message["role"])
		content := stringValue(message["content"])

		switch role {
		case "system":
			converted = append(converted, openai.SystemMessage(content))
		case "assistant":
			// message["tool_calls"]
			// {
			// 	"role": "assistant",
			// 	"content": "",
			// 	"tool_calls": [
			// 	  {
			// 		"id": "call_1",
			// 		"type": "function",
			// 		"function": {
			// 			"name": "search_web",
			// 			"arguments": "{\"query\":\"Go 1.24\"}"
			// 		}
			// 	},
			// 	...
			// 	]
			// }
			toolCalls, err := convertMessageToolCalls(message["tool_calls"])
			if err != nil {
				return nil, err
			}
			if len(toolCalls) == 0 {
				assistant := openai.AssistantMessage(content)
				if assistant.OfAssistant != nil {
					if name := stringValue(message["name"]); name != "" {
						assistant.OfAssistant.Name = openai.String(name)
					}
				}
				converted = append(converted, assistant)
				continue
			}

			assistant := openai.ChatCompletionAssistantMessageParam{
				ToolCalls: toolCalls,
			}
			if content != "" {
				assistant.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
					OfString: openai.String(content),
				}
			}
			if name := stringValue(message["name"]); name != "" {
				assistant.Name = openai.String(name)
			}
			converted = append(converted, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &assistant,
			})
		case "tool":
			toolCallID := stringValue(message["tool_call_id"])
			if toolCallID == "" {
				return nil, errors.New("tool message missing tool_call_id")
			}
			converted = append(converted, openai.ToolMessage(content, toolCallID))
		default:
			converted = append(converted, openai.UserMessage(content))
		}
	}

	return converted, nil
}

func convertMessageToolCalls(value any) ([]openai.ChatCompletionMessageToolCallParam, error) {
	switch raw := value.(type) {
	case nil:
		return nil, nil
	case []*model.ToolCall: // 模型返回的需要调用的工具列表
		out := make([]openai.ChatCompletionMessageToolCallParam, 0, len(raw))
		for _, call := range raw {
			if call == nil {
				continue
			}
			argsJSON, err := normalizeArguments(call.Args)
			if err != nil {
				return nil, err
			}
			out = append(out, openai.ChatCompletionMessageToolCallParam{
				ID: call.ID,
				Function: openai.ChatCompletionMessageToolCallFunctionParam{
					Name:      call.Name,
					Arguments: argsJSON,
				},
			})
		}
		return out, nil
	case []map[string]any:
		out := make([]openai.ChatCompletionMessageToolCallParam, 0, len(raw))
		// 处理工具定义时
		// 每个item是一个tool的OpenAI schema格式，例如：
		//
		// {
		// 	"type": "function",
		// 	"function": {
		// 		"name": "search",
		// 		"description": "Search the web for information",
		// 		"parameters": json.RawMessage(`{
		// 			"type": "object",
		// 			"properties": {
		// 				"n": {
		// 					"type": "number",
		// 					"description": "The number of results to return"
		// 				}
		// 			}
		// 		}`)
		// 	}
		// }
		// 处理工具调用的历史信息时
		// message["tool_calls"]
		// {
		// 	"role": "assistant",
		// 	"content": "",
		// 	"tool_calls": [
		// 	  {
		// 		"id": "call_1",
		// 		"type": "function",
		// 		"function": {
		// 			"name": "search_web",
		// 			"arguments": "{\"query\":\"Go 1.24\"}"
		// 		}
		// 	},
		// 	...
		// 	]
		// }
		for _, item := range raw {
			call, err := convertMessageToolCallMap(item)
			if err != nil {
				return nil, err
			}
			out = append(out, call)
		}
		return out, nil
	case []any:
		out := make([]openai.ChatCompletionMessageToolCallParam, 0, len(raw))
		for _, item := range raw {
			switch value := item.(type) {
			case map[string]any:
				call, err := convertMessageToolCallMap(value)
				if err != nil {
					return nil, err
				}
				out = append(out, call)
			case *model.ToolCall:
				if value == nil {
					continue
				}
				argsJSON, err := normalizeArguments(value.Args)
				if err != nil {
					return nil, err
				}
				out = append(out, openai.ChatCompletionMessageToolCallParam{
					ID: value.ID,
					Function: openai.ChatCompletionMessageToolCallFunctionParam{
						Name:      value.Name,
						Arguments: argsJSON,
					},
				})
			default:
				return nil, fmt.Errorf("unsupported tool call type %T", item)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported tool_calls type %T", value)
	}
}

// convertMessageToolCallMap converts a tool call map(OpenAI function schema format) to `ChatCompletionMessageToolCallParam`.
// Args:
//
//	item: map[string]any
//
// Returns:
//
//	openai.ChatCompletionMessageToolCallParam
//
//	error
func convertMessageToolCallMap(item map[string]any) (openai.ChatCompletionMessageToolCallParam, error) {
	function, _ := item["function"].(map[string]any)

	name := stringValue(item["name"])
	if function != nil {
		if v := stringValue(function["name"]); v != "" {
			name = v
		}
	}
	if name == "" {
		return openai.ChatCompletionMessageToolCallParam{}, errors.New("tool call name is required")
	}

	argumentsSource := item["arguments"]
	if function != nil {
		argumentsSource = function["arguments"]
	}
	argumentsJSON, err := normalizeArguments(argumentsSource)
	if err != nil {
		return openai.ChatCompletionMessageToolCallParam{}, err
	}

	id := stringValue(item["id"])
	if id == "" {
		return openai.ChatCompletionMessageToolCallParam{}, errors.New("tool call id is required")
	}

	return openai.ChatCompletionMessageToolCallParam{
		ID: id,
		Function: openai.ChatCompletionMessageToolCallFunctionParam{
			Name:      name,
			Arguments: argumentsJSON,
		},
	}, nil
}

// convertTools converts a list of tool definitions(OpenAI function schema format) to `ChatCompletionToolParam`.
// Args:
//
//	tools: []map[string]any
//
// Returns:
//
//	[]openai.ChatCompletionToolParam
//
//	error
//
// Example:
//
//	[
//		{
//			"type": "function",
//			"function": {
//				"name": "search",
//				"description": "Search the web for information",
//				"parameters": json.RawMessage(`{
//					"type": "object",
//					"properties": {
//						"n": {
//							"type": "number",
//							"description": "The number of results to return"
//						}
//					}
//				}
//			}
//		}
//	]
func convertTools(tools []map[string]any) ([]openai.ChatCompletionToolParam, error) {
	out := make([]openai.ChatCompletionToolParam, 0, len(tools))

	for _, tool := range tools {
		function, ok := tool["function"].(map[string]any)
		if !ok {
			return nil, errors.New("tool definition missing function object")
		}

		name := stringValue(function["name"])
		if name == "" {
			return nil, errors.New("tool definition name is required")
		}

		definition := shared.FunctionDefinitionParam{
			Name: name,
		}
		if description := stringValue(function["description"]); description != "" {
			definition.Description = openai.String(description)
		}

		parameters, err := convertFunctionParameters(function["parameters"])
		if err != nil {
			return nil, fmt.Errorf("tool %s parameters: %w", name, err)
		}
		if len(parameters) > 0 {
			definition.Parameters = parameters
		}

		out = append(out, openai.ChatCompletionToolParam{
			Function: definition,
		})
	}

	return out, nil
}

// convertFunctionParameters converts a function parameters(json.RawMessage) to `shared.FunctionParameters`.
func convertFunctionParameters(value any) (shared.FunctionParameters, error) {
	switch raw := value.(type) {
	case json.RawMessage:
		if len(raw) == 0 {
			return nil, nil
		}
		var params shared.FunctionParameters
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		return params, nil
	default:
		return nil, fmt.Errorf("unsupported function parameters type %T", value)
	}
}

func normalizeArguments(value any) (string, error) {
	switch raw := value.(type) {
	case nil:
		return "{}", nil
	case string:
		if strings.TrimSpace(raw) == "" {
			return "{}", nil
		}
		return raw, nil
	case json.RawMessage:
		if len(raw) == 0 {
			return "{}", nil
		}
		return string(raw), nil
	case []byte:
		if len(raw) == 0 {
			return "{}", nil
		}
		return string(raw), nil
	default:
		encoded, err := json.Marshal(raw)
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

// extractExtraString 从 SDK 响应的 ExtraFields 中提取字符串类型的厂商扩展字段。
//
// 用于解析 Qwen3 的 reasoning_content、DeepSeek 的 reasoning_content 等。
// ExtraFields 是 openai-go SDK 预留的扩展机制：
// - SDK 只解析 OpenAI 标准字段（content, tool_calls 等）
// - 厂商扩展字段落入 ExtraFields map
// - 通过 Raw() 拿到原始 JSON 值，再反序列化为目标类型
func extractExtraString(extraFields map[string]respjson.Field, key string) string {
	field, ok := extraFields[key]
	if !ok {
		return ""
	}
	// 注意：不依赖 field.Valid()，因为某些情况下 Valid() 返回 false 但 Raw() 仍有值
	raw := field.Raw()
	if raw == "" || raw == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return ""
	}
	return s
}
