package model

// ToolCall represents a tool call in the LLM response.
type ToolCall struct {
	ID   string // OpenAI 接口返回，把工具执行结果和调用对应起来。会把它放到 tool_call_id 里回传给模型
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
