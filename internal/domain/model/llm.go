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

// StreamEvent 表示流式响应中的一个增量事件
//
// 为什么用 Kind 区分而不是定义多个子类型：
// - 流事件的种类有限且稳定（文本片段、工具调用、结束、错误）
// - 用 Kind 字段可以让消费者在一个 switch 里处理所有情况
// - 避免为 4 种事件各建一个 struct + interface，过度设计
type StreamEvent struct {
	Kind StreamEventKind
	// Delta 是本次事件携带的文本增量（仅 Kind == StreamEventDelta 时有意义）
	Delta string
	// Response 是流结束时的完整响应（仅 Kind == StreamEventDone 时有意义）
	// 包含完整 Content、ToolCalls、token 用量等信息
	Response *LLMResponse
	// Err 是流中出现的错误（仅 Kind == StreamEventError 时有意义）
	Err error
}

type StreamEventKind int

const (
	StreamEventDelta StreamEventKind = iota // 模型增量输出
	StreamEventDone                         // 流正常结束，模型输出完成
	StreamEventError                        // 流异常结束，模型输出出错
)
