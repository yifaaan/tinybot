package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httputil"
	"strings"
	"tinybot/internal/domain/model"
	"tinybot/internal/utils/logger"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
)

const (
	defaultOpenAIResponsesBaseURL = "https://api.openai.com/v1"
	defaultOpenAIResponsesModel   = "gpt-4.1-mini"
)

type OpenAIResponsesProvider struct {
	client           *openai.Client
	model            string
	enableThinking   bool
	reasoningEffort  string
	reasoningSummary string
	textVerbosity    string
}

type responsesToolCallAccum struct {
	ID   string
	Name string
	Args strings.Builder
}

func NewOpenAIResponsesProvider(apiKey string, apiBase string, model string, options Options) (*OpenAIResponsesProvider, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("openai-responses provider: api key is required")
	}
	if strings.TrimSpace(apiBase) == "" {
		apiBase = defaultOpenAIResponsesBaseURL
	}
	if strings.TrimSpace(model) == "" {
		model = defaultOpenAIResponsesModel
	}

	logger.Info("openai-responses provider initialized", "model", model, "api_base", apiBase)

	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(apiBase))
	return &OpenAIResponsesProvider{
		client:           &client,
		model:            model,
		enableThinking:   options.EnableThinking,
		reasoningEffort:  normalizeReasoningEffort(options.ReasoningEffort),
		reasoningSummary: normalizeReasoningSummary(options.ReasoningSummary),
		textVerbosity:    normalizeVerbosity(options.TextVerbosity),
	}, nil
}

func (p *OpenAIResponsesProvider) Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error) {
	input, err := convertResponsesInput(messages)
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("openai-responses provider convert messages: %w", err)
	}

	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(p.model),
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: input,
		},
	}
	if p.enableThinking {
		params.Reasoning = buildReasoningParam(p.reasoningEffort, p.reasoningSummary)
	}
	if maxTokens > 0 {
		params.MaxOutputTokens = openai.Int(int64(maxTokens))
	}
	if len(tools) > 0 {
		convertedTools, err := convertResponsesTools(tools)
		if err != nil {
			return model.LLMResponse{}, fmt.Errorf("openai-responses provider convert tools: %w", err)
		}
		params.Tools = convertedTools
	}

	reqOpts := p.responsesRequestOptions()
	resp, err := p.client.Responses.New(ctx, params, reqOpts...)
	if err != nil {
		return model.LLMResponse{}, fmt.Errorf("openai-responses provider create response: %s", formatOpenAIResponsesError(err))
	}

	out := model.LLMResponse{
		Content:      strings.TrimSpace(resp.OutputText()),
		FinishReason: string(resp.Status),
		PromptTokens: int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
	}

	for _, item := range resp.Output {
		switch item.Type {
		case "function_call":
			args := make(map[string]any)
			rawArgs := strings.TrimSpace(item.Arguments)
			if rawArgs != "" {
				if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
					args = map[string]any{"raw": rawArgs}
				}
			}
			out.ToolCalls = append(out.ToolCalls, &model.ToolCall{
				ID:   firstNonEmpty(item.CallID, item.ID),
				Name: item.Name,
				Args: args,
			})
		case "reasoning":
			if text := extractResponseReasoning(item); text != "" {
				out.Thinking = strings.TrimSpace(strings.Join([]string{out.Thinking, text}, "\n"))
			}
		}
	}
	if out.Thinking == "" && resp.Usage.OutputTokensDetails.ReasoningTokens > 0 {
		out.Thinking = fmt.Sprintf(
			"Provider used reasoning internally (%d reasoning tokens), but did not expose a readable summary.",
			resp.Usage.OutputTokensDetails.ReasoningTokens,
		)
	}

	if out.Content == "" && len(out.ToolCalls) == 0 {
		return model.LLMResponse{}, errors.New("openai-responses provider: empty response")
	}
	return out, nil
}

func (p *OpenAIResponsesProvider) ChatStream(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) <-chan model.StreamEvent {
	ch := make(chan model.StreamEvent)

	go func() {
		defer close(ch)

		input, err := convertResponsesInput(messages)
		if err != nil {
			ch <- model.StreamEvent{Kind: model.StreamEventError, Err: fmt.Errorf("openai-responses provider convert messages: %w", err)}
			return
		}

		params := responses.ResponseNewParams{
			Model: shared.ResponsesModel(p.model),
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: input,
			},
		}
		if p.enableThinking {
			params.Reasoning = buildReasoningParam(p.reasoningEffort, p.reasoningSummary)
		}
		if maxTokens > 0 {
			params.MaxOutputTokens = openai.Int(int64(maxTokens))
		}
		if len(tools) > 0 {
			convertedTools, err := convertResponsesTools(tools)
			if err != nil {
				ch <- model.StreamEvent{Kind: model.StreamEventError, Err: fmt.Errorf("openai-responses provider convert tools: %w", err)}
				return
			}
			params.Tools = convertedTools
		}

		reqOpts := p.responsesRequestOptions()
		stream := p.client.Responses.NewStreaming(ctx, params, reqOpts...)

		var contentBuilder strings.Builder
		var thinkingBuilder strings.Builder
		accum := make(map[string]*responsesToolCallAccum)
		var finalResp *responses.Response

		for stream.Next() {
			event := stream.Current()

			// Debug log all event types
			logger.Debug("openai-responses stream event", "type", event.Type, "item_type", event.Item.Type, "item_id", event.ItemID)

			switch event.Type {
			case "response.output_text.delta":
				delta := event.Delta.OfString
				if delta != "" {
					contentBuilder.WriteString(delta)
					ch <- model.StreamEvent{Kind: model.StreamEventDelta, Delta: delta}
				}
			case "response.reasoning_summary_text.delta", "response.reasoning_summary.delta", "response.reasoning.delta":
				delta := event.Delta.OfString
				if delta != "" {
					thinkingBuilder.WriteString(delta)
					ch <- model.StreamEvent{Kind: model.StreamEventThinking, Delta: delta}
				}
			case "response.function_call_arguments.delta":
				callID := strings.TrimSpace(event.ItemID)
				if callID == "" {
					callID = strings.TrimSpace(event.Item.CallID)
				}
				if callID == "" {
					callID = fmt.Sprintf("call-%d", event.OutputIndex)
				}
				call := accum[callID]
				if call == nil {
					call = &responsesToolCallAccum{
						ID:   callID,
						Name: strings.TrimSpace(event.Item.Name),
					}
					accum[callID] = call
				}
				if call.Name == "" {
					call.Name = strings.TrimSpace(event.Item.Name)
				}
				delta := event.Delta.OfString
				if strings.TrimSpace(delta) != "" {
					call.Args.WriteString(delta)
				}
			case "response.function_call_arguments.done":
				callID := firstNonEmpty(event.ItemID, event.Item.CallID)
				if callID == "" {
					callID = fmt.Sprintf("call-%d", event.OutputIndex)
				}
				call := accum[callID]
				if call == nil {
					call = &responsesToolCallAccum{ID: callID}
					accum[callID] = call
				}
				if call.Name == "" {
					call.Name = strings.TrimSpace(event.Item.Name)
				}
				if strings.TrimSpace(event.Arguments) != "" {
					call.Args.Reset()
					call.Args.WriteString(event.Arguments)
				}
			case "response.output_item.done":
				if event.Item.Type == "function_call" {
					callID := firstNonEmpty(event.Item.CallID, event.ItemID, event.Item.ID)
					if callID == "" {
						callID = fmt.Sprintf("call-%d", event.OutputIndex)
					}
					call := accum[callID]
					if call == nil {
						call = &responsesToolCallAccum{ID: callID}
						accum[callID] = call
					}
					if call.Name == "" {
						call.Name = strings.TrimSpace(event.Item.Name)
					}
					if strings.TrimSpace(event.Item.Arguments) != "" {
						call.Args.Reset()
						call.Args.WriteString(event.Item.Arguments)
					}
				}
				// Handle reasoning item with summary text
				if event.Item.Type == "reasoning" {
					logger.Debug("openai-responses reasoning item", "summary_count", len(event.Item.Summary), "summary", fmt.Sprintf("%+v", event.Item.Summary))
					if len(event.Item.Summary) > 0 {
						var summaryText strings.Builder
						for _, s := range event.Item.Summary {
							if s.Text != "" {
								summaryText.WriteString(s.Text)
								summaryText.WriteString("\n")
							}
						}
						text := summaryText.String()
						if text != "" {
							logger.Debug("openai-responses reasoning text", "text_len", len(text))
							thinkingBuilder.WriteString(text)
							ch <- model.StreamEvent{Kind: model.StreamEventThinking, Delta: text}
						}
					}
				}
			case "response.completed":
				finalResp = &event.Response
				// Debug: log the full response output to see reasoning content
				for i, item := range event.Response.Output {
					logger.Debug("openai-responses response.output", "index", i, "type", item.Type, "summary_count", len(item.Summary), "summary", fmt.Sprintf("%+v", item.Summary))
				}
			case "response.failed", "error":
				errText := firstNonEmpty(event.Message, "responses stream failed")
				ch <- model.StreamEvent{Kind: model.StreamEventError, Err: errors.New(errText)}
				return
			}
		}

		if err := stream.Err(); err != nil {
			// If we have partial content when stream fails, include it in the error message
			// but still return an error so the caller knows the response is incomplete
			partialContent := strings.TrimSpace(contentBuilder.String())
			partialThinking := strings.TrimSpace(thinkingBuilder.String())
			logger.Warn("stream error with partial content",
				"error", err,
				"partial_content_len", len(partialContent),
				"partial_thinking_len", len(partialThinking))
			ch <- model.StreamEvent{Kind: model.StreamEventError, Err: fmt.Errorf("openai-responses provider stream: %s", formatOpenAIResponsesError(err))}
			return
		}

		var out model.LLMResponse
		if finalResp != nil {
			parsed, err := parseResponsesResponse(finalResp)
			if err != nil {
				ch <- model.StreamEvent{Kind: model.StreamEventError, Err: err}
				return
			}
			out = parsed
		} else {
			out = model.LLMResponse{
				Content:  strings.TrimSpace(contentBuilder.String()),
				Thinking: strings.TrimSpace(thinkingBuilder.String()),
			}
		}

		if out.Content == "" && contentBuilder.Len() > 0 {
			out.Content = strings.TrimSpace(contentBuilder.String())
		}
		// If we got thinking from final response but didn't stream it, send it now
		if out.Thinking != "" && thinkingBuilder.Len() == 0 {
			ch <- model.StreamEvent{Kind: model.StreamEventThinking, Delta: out.Thinking}
		}
		if out.Thinking == "" && thinkingBuilder.Len() > 0 {
			out.Thinking = strings.TrimSpace(thinkingBuilder.String())
		}
		if len(out.ToolCalls) == 0 && len(accum) > 0 {
			out.ToolCalls = toolCallsFromAccum(accum)
		}

		if out.Content == "" && len(out.ToolCalls) == 0 {
			ch <- model.StreamEvent{Kind: model.StreamEventError, Err: errors.New("openai-responses provider: empty stream response")}
			return
		}
		ch <- model.StreamEvent{Kind: model.StreamEventDone, Response: &out}
	}()

	return ch
}

func convertResponsesInput(messages []map[string]any) ([]responses.ResponseInputItemUnionParam, error) {
	out := make([]responses.ResponseInputItemUnionParam, 0, len(messages))

	for _, message := range messages {
		role := stringValue(message["role"])
		content := strings.TrimSpace(stringValue(message["content"]))

		switch role {
		case model.RoleSystem:
			if content != "" {
				out = append(out, responses.ResponseInputItemParamOfMessage(content, responses.EasyInputMessageRoleSystem))
			}
		case model.RoleAssistant:
			if content != "" {
				out = append(out, responses.ResponseInputItemParamOfMessage(content, responses.EasyInputMessageRoleAssistant))
			}
			toolCalls, err := convertResponsesToolCalls(message["tool_calls"])
			if err != nil {
				return nil, err
			}
			out = append(out, toolCalls...)
		case model.RoleTool:
			toolCallID := strings.TrimSpace(stringValue(message["tool_call_id"]))
			if toolCallID == "" {
				continue
			}
			out = append(out, responses.ResponseInputItemParamOfFunctionCallOutput(toolCallID, content))
		default:
			if content != "" {
				out = append(out, responses.ResponseInputItemParamOfMessage(content, responses.EasyInputMessageRoleUser))
			}
		}
	}

	return out, nil
}

func convertResponsesToolCalls(value any) ([]responses.ResponseInputItemUnionParam, error) {
	switch raw := value.(type) {
	case nil:
		return nil, nil
	case []map[string]any:
		out := make([]responses.ResponseInputItemUnionParam, 0, len(raw))
		for _, item := range raw {
			call, err := convertResponsesToolCallMap(item)
			if err != nil {
				return nil, err
			}
			out = append(out, call)
		}
		return out, nil
	case []any:
		out := make([]responses.ResponseInputItemUnionParam, 0, len(raw))
		for _, item := range raw {
			callMap, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("unsupported tool call item type %T", item)
			}
			call, err := convertResponsesToolCallMap(callMap)
			if err != nil {
				return nil, err
			}
			out = append(out, call)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported tool_calls type %T", value)
	}
}

func convertResponsesToolCallMap(item map[string]any) (responses.ResponseInputItemUnionParam, error) {
	id := strings.TrimSpace(stringValue(item["id"]))
	if id == "" {
		id = strings.TrimSpace(stringValue(item["tool_call_id"]))
	}

	name := strings.TrimSpace(stringValue(item["name"]))
	argsValue := item["arguments"]

	if function, ok := item["function"].(map[string]any); ok {
		if name == "" {
			name = strings.TrimSpace(stringValue(function["name"]))
		}
		if argsValue == nil {
			argsValue = function["arguments"]
		}
	}

	if id == "" || name == "" {
		return responses.ResponseInputItemUnionParam{}, errors.New("tool call requires id and name")
	}

	args, err := normalizeArguments(argsValue)
	if err != nil {
		return responses.ResponseInputItemUnionParam{}, err
	}
	return responses.ResponseInputItemParamOfFunctionCall(args, id, name), nil
}

func convertResponsesTools(tools []map[string]any) ([]responses.ToolUnionParam, error) {
	out := make([]responses.ToolUnionParam, 0, len(tools))

	for _, tool := range tools {
		function, ok := tool["function"].(map[string]any)
		if !ok {
			return nil, errors.New("tool definition missing function object")
		}

		name := stringValue(function["name"])
		if name == "" {
			return nil, errors.New("tool definition name is required")
		}

		parameters, err := convertFunctionParameters(function["parameters"])
		if err != nil {
			return nil, fmt.Errorf("tool %s parameters: %w", name, err)
		}
		parameters = ensureResponsesStrictSchema(parameters)

		definition := responses.FunctionToolParam{
			Name:       name,
			Parameters: parameters,
			Strict:     openai.Bool(false),
		}
		if description := stringValue(function["description"]); description != "" {
			definition.Description = openai.String(description)
		}

		out = append(out, responses.ToolUnionParam{OfFunction: &definition})
	}

	return out, nil
}

func extractResponseReasoning(item responses.ResponseOutputItemUnion) string {
	if item.Type != "reasoning" {
		return ""
	}

	parts := make([]string, 0, len(item.Summary))
	for _, summary := range item.Summary {
		if text := strings.TrimSpace(summary.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func buildReasoningParam(effort string, summary string) shared.ReasoningParam {
	param := shared.ReasoningParam{}
	switch effort {
	case "low":
		param.Effort = shared.ReasoningEffortLow
	case "medium":
		param.Effort = shared.ReasoningEffortMedium
	case "high":
		param.Effort = shared.ReasoningEffortHigh
	}

	switch summary {
	case "auto":
		param.Summary = shared.ReasoningSummaryAuto
	case "concise":
		param.Summary = shared.ReasoningSummaryConcise
	case "detailed":
		param.Summary = shared.ReasoningSummaryDetailed
	}

	return param
}

func (p *OpenAIResponsesProvider) responsesRequestOptions() []option.RequestOption {
	opts := make([]option.RequestOption, 0, 1)
	if p.textVerbosity != "" {
		opts = append(opts, option.WithJSONSet("text.verbosity", p.textVerbosity))
	}
	return opts
}

func normalizeReasoningEffort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "high"
	}
}

func normalizeReasoningSummary(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto", "concise", "detailed":
		return strings.ToLower(strings.TrimSpace(value))
	case "off":
		return ""
	default:
		return "detailed"
	}
}

func normalizeVerbosity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "medium"
	}
}

func ensureResponsesStrictSchema(value shared.FunctionParameters) shared.FunctionParameters {
	root := map[string]any(value)
	applyStrictObjectSchema(root)
	return shared.FunctionParameters(root)
}

func applyStrictObjectSchema(node map[string]any) {
	if strings.EqualFold(stringValue(node["type"]), "object") {
		if _, ok := node["additionalProperties"]; !ok {
			node["additionalProperties"] = false
		}
	}

	properties, ok := node["properties"].(map[string]any)
	if !ok {
		return
	}
	for _, raw := range properties {
		child, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		applyStrictObjectSchema(child)
	}
}

func parseResponsesResponse(resp *responses.Response) (model.LLMResponse, error) {
	if resp == nil {
		return model.LLMResponse{}, errors.New("openai-responses provider: nil response")
	}

	out := model.LLMResponse{
		Content:      strings.TrimSpace(resp.OutputText()),
		FinishReason: string(resp.Status),
		PromptTokens: int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
	}

	for _, item := range resp.Output {
		switch item.Type {
		case "function_call":
			args := make(map[string]any)
			rawArgs := strings.TrimSpace(item.Arguments)
			if rawArgs != "" {
				if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
					args = map[string]any{"raw": rawArgs}
				}
			}
			out.ToolCalls = append(out.ToolCalls, &model.ToolCall{
				ID:   firstNonEmpty(item.CallID, item.ID),
				Name: item.Name,
				Args: args,
			})
		case "reasoning":
			if text := extractResponseReasoning(item); text != "" {
				if out.Thinking != "" {
					out.Thinking += "\n"
				}
				out.Thinking += strings.TrimSpace(text)
			}
		}
	}
	if out.Thinking == "" && resp.Usage.OutputTokensDetails.ReasoningTokens > 0 {
		out.Thinking = fmt.Sprintf(
			"Provider used reasoning internally (%d reasoning tokens), but did not expose a readable summary.",
			resp.Usage.OutputTokensDetails.ReasoningTokens,
		)
	}

	return out, nil
}

func toolCallsFromAccum(accum map[string]*responsesToolCallAccum) []*model.ToolCall {
	out := make([]*model.ToolCall, 0, len(accum))
	for _, call := range accum {
		args := make(map[string]any)
		rawArgs := strings.TrimSpace(call.Args.String())
		if rawArgs != "" {
			if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
				args = map[string]any{"raw": rawArgs}
			}
		}
		out = append(out, &model.ToolCall{
			ID:   call.ID,
			Name: call.Name,
			Args: args,
		})
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func formatOpenAIResponsesError(err error) string {
	if err == nil {
		return ""
	}

	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		raw := strings.TrimSpace(apiErr.RawJSON())
		if raw != "" {
			return raw
		}
		if apiErr.Response != nil {
			if dump, dumpErr := httputil.DumpResponse(apiErr.Response, true); dumpErr == nil {
				if text := strings.TrimSpace(string(dump)); text != "" {
					return text
				}
			}
		}
	}

	return err.Error()
}
