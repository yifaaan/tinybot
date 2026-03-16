package model

// ToolCall represents a tool call in the LLM response.
type ToolCall struct {
	ID   string // OpenAI 接口返回，把工具执行结果和调用对应起来。会把它放到 tool_call_id 里回传给模型
	Name string
	Args map[string]any
}

// LLMResponse 是一次模型调用的标准化返回。
//
// Thinking 字段：
// - 现代推理模型（Qwen3、DeepSeek R1）会先输出一段"思考过程"，再输出正式回答
// - Thinking 存放这段推理文本，Content 只存最终回答
// - 不支持 thinking 的模型/provider 中，Thinking 为零值空串，完全向后兼容
// - Thinking 不应回传给模型作为历史上下文（会浪费 token 且可能干扰后续推理）
type LLMResponse struct {
	Content      string
	Thinking     string // 模型推理过程（Qwen3 thinking / DeepSeek reasoning），可选
	ToolCalls    []*ToolCall
	FinishReason string
	PromptTokens int
	OutputTokens int
}

// HasToolCalls reports whether the LLM response contains tool calls.
func (r *LLMResponse) HasToolCalls() bool {
	return len(r.ToolCalls) > 0
}

// StreamEvent 表示流式响应中的一个增量事件。
//
// 为什么用 Kind 区分而不是定义多个子类型：
// - 流事件的种类有限且稳定（推理片段、正文片段、结束、错误）
// - 用 Kind 字段可以让消费者在一个 switch 里处理所有情况
// - 避免为每种事件各建一个 struct + interface，过度设计
type StreamEvent struct {
	Kind StreamEventKind
	// Delta 是本次事件携带的文本增量
	// Kind == StreamEventDelta 时为正文片段
	// Kind == StreamEventThinking 时为推理过程片段
	Delta string
	// Response 是流结束时的完整响应（仅 Kind == StreamEventDone 时有意义）
	// 包含完整 Content、Thinking、ToolCalls、token 用量等信息
	Response *LLMResponse
	// Err 是流中出现的错误（仅 Kind == StreamEventError 时有意义）
	Err error
}

// StreamEventKind 枚举流事件类型。
//
// 顺序反映了实际流的时序：thinking 先于 content，content 先于 done/error。
// 所有消费方都通过命名常量引用，不依赖数字值，因此插入新类型是安全的。
type StreamEventKind int

const (
	StreamEventDelta    StreamEventKind = iota // 正文增量输出
	StreamEventThinking                        // 推理过程增量输出（Qwen3 thinking / DeepSeek reasoning）
	StreamEventDone                            // 流正常结束，模型输出完成
	StreamEventError                           // 流异常结束，模型输出出错
)
