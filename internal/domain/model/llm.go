package model

import "encoding/json"

type ToolCall struct {
	ID   string
	Name string
	Args json.RawMessage
}

type LLMResponse struct {
	Content      string
	ToolCalls    []*ToolCall
	FinishReason string
	PromptTokens int
	OutputTokens int
}

// HasToolCalls reports whether the LLM response contains tool calls.
func (r LLMResponse) HasToolCalls() bool {
	return len(r.ToolCalls) > 0
}
