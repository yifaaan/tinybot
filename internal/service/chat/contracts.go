// Package chat 定义单轮 agent 对话的服务边界。
//
// 这一层只关心“如何完成一次 agent turn”，不关心：
// - session 最终落成 JSONL、数据库还是其他格式
// - provider 背后接的是哪家模型服务
// - workspace 文件到底如何读取和组织
//
// 这样分层的原因是：agent turn 是整个 tinybot 最稳定、最值得优先保护的行为核心。
// 只要这一层边界清晰，CLI、gateway、cron、heartbeat 都可以复用同一套业务逻辑。
package chat

import (
	"context"

	"tinybot/internal/domain/model"
)

// SessionRepository 定义了 chat service 依赖的最小会话持久化边界。
//
// 为什么接口只保留“取/建”和“保存”两个动作：
// - chat service 只关心一次对话回合开始时拿到 session，结束时把结果写回去
// - 文件路径规则、JSONL 结构、缓存策略都属于 repository 内部实现细节
// - 接口越小，后续替换存储方式时，service 层需要改的地方越少
type SessionRepository interface {
	// GetOrCreateSession 根据稳定的会话 key 返回已有 session，不存在时创建新的 session。
	//
	// 参数：
	// - ctx: 请求级上下文，用于取消、超时和未来的 tracing
	// - key: 稳定的会话标识，决定多轮对话是否会归到同一个线程
	//
	// 返回：
	// - *model.Session: 可变的领域会话对象
	// - error: 读取或创建失败时返回错误
	GetOrCreateSession(ctx context.Context, key string) (*model.Session, error)

	// SaveSession 持久化当前会话的完整 trace。
	//
	// 参数：
	// - ctx: 请求级上下文
	// - session: 需要保存的会话
	//
	// 返回：
	// - error: 持久化失败时返回错误
	SaveSession(ctx context.Context, session *model.Session) error
}

// CompletionClient 抽象了“向模型发起一次对话请求”的能力。
//
// 为什么要把 provider SDK 隔在这个接口后面：
// - 第三方模型 SDK 的变化速度通常比业务逻辑快得多
// - agent turn 的编排逻辑不应该知道 provider 的私有类型和调用细节
// - 测试时也可以用假的 CompletionClient 完全替代真实网络调用
type CompletionClient interface {
	// Chat 向模型发送消息和工具定义，返回一次标准化后的模型响应。
	//
	// 参数：
	// 	- ctx: 请求上下文
	// 	- messages: 模型输入消息列表
	// 	- tools: 暴露给模型的工具 schema 列表
	// 	- maxTokens: 本次生成的 token 上限
	// 	- temperature: 采样温度
	//
	// 返回：
	// - model.LLMResponse: 统一后的模型返回
	// - error: 调用 provider 失败时返回错误
	Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error)
}

// ToolExecutor 暴露 chat service 需要的工具能力：
// 既要把工具 schema 告诉模型，也要能真正执行模型选中的工具。
//
// 为什么把“定义”和“执行”放在同一个边界：
// - 可以确保模型看到的工具名、参数结构和运行时真实实现保持一致
// - 避免 schema 和实现分散后逐渐漂移，最后出现“模型能看到，但运行不了”的工具
type ToolExecutor interface {
	// GetDefinitions 返回当前可供模型调用的工具定义。
	//
	// 返回：
	// - []map[string]any: provider 可直接消费的工具 schema 列表
	GetDefinitions() []map[string]any

	// Execute 执行指定工具。
	//
	// 参数：
	// - ctx: 请求上下文
	// - name: 工具名
	// - params: 模型生成的参数
	//
	// 返回：
	// - string: 工具执行结果文本，会被重新送回模型
	// - error: 工具执行失败时返回错误
	Execute(ctx context.Context, name string, params map[string]any) (string, error)
}

// PromptBuilder 负责把“会话历史 + 当前输入 + workspace 上下文”组装成模型可读消息。
//
// 之所以把 prompt 装配单独拆成接口，而不是塞进 chat service：
// - prompt 的顺序和降级策略本身就是值得单独保护的行为
// - 这部分逻辑经常变化，也更适合被细粒度测试覆盖
// - chat service 应该只负责编排，不应该知道 workspace 文件是如何读取和拼接的
type PromptBuilder interface {
	// BuildMessages 生成一次模型调用所需的完整消息列表。
	//
	// 参数：
	// - history: 历史消息
	// - currentMessage: 当前用户输入
	// - skillNames: 当前轮显式选中的技能名；当前实现保留了这个入口，但尚未真正使用
	//
	// 返回：
	// - []map[string]any: 提供给模型的消息序列
	BuildMessages(history []*model.Message, currentMessage string, skillNames []string) []map[string]any

	// AddAssistantMessage 把 assistant 的普通回复或 tool-call 痕迹追加到消息列表。
	//
	// 参数：
	// - messages: 现有消息
	// - content: assistant 文本内容
	// - toolCalls: assistant 发起的工具调用描述
	//
	// 返回：
	// - []map[string]any: 追加后的消息列表
	AddAssistantMessage(messages []map[string]any, content string, toolCalls []map[string]any) []map[string]any

	// AddToolResult 把某次工具执行结果追加到消息列表。
	//
	// 参数：
	// - messages: 现有消息
	// - toolCallID: 对应的 tool call ID
	// - toolName: 工具名
	// - result: 工具结果文本
	//
	// 返回：
	// - []map[string]any: 追加后的消息列表
	AddToolResult(messages []map[string]any, toolCallID string, toolName string, result string) []map[string]any
}
