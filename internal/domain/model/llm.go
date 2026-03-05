package model

type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

type LLMResponse struct {
	Content      string
	ToolCalls    []*ToolCall
	FinishReason string
	PromptTokens int
	OutputTokens int
}

// HasToolCalls reports whether the LLM response contains tool calls.
func (r *LLMResponse) HasToolCalls() bool {
	return len(r.ToolCalls) > 0
}
