package chat

import (
	"fmt"
	"strings"
	"time"

	"tinybot/internal/domain/model"
)

var defaultBootstrapFiles = []string{
	"AGENTS.md",
	"SOUL.md",
	"USER.md",
	"TOOLS.md",
	"IDENTITY.md",
}

// BootstrapReader 负责读取 workspace 里的静态引导文档。
//
// 为什么不让 Builder 直接自己读文件：
// - Builder 更适合专注于“拼接规则”和“上下文顺序”
// - 文件系统访问细节更适合留在 adapter 层
// - 这样测试时也可以用 stub 快速验证 prompt 行为，而不必依赖真实磁盘
type BootstrapReader interface {
	// Load 读取给定文件名列表。
	//
	// 参数：
	//  - names: 需要读取的 bootstrap 文件名列表
	//
	// 返回：
	//  - map[string]string: 文件名到内容的映射；缺失文件可以不返回
	//  - error: 读取过程的整体错误
	Load(names []string) (map[string]string, error)
}

// MemoryReader 暴露格式化后的 memory 上下文。
//
// 这里故意让接口直接返回“可拼接文本”，而不是原始文件列表，
// 是为了把“如何组织 MEMORY.md 和 daily notes”这件事留在 adapter 层。
type MemoryReader interface {
	// BuildContext 构建 memory 区块文本。
	//
	// 返回：
	// - string: 适合直接拼进 system prompt 的 memory 内容；没有内容时可返回空串
	BuildContext() string
}

// SkillCatalog 暴露技能系统需要的最小能力。
//
// 当前 prompt 里的技能分成两类：
// - always skills：直接把全文注入 prompt
// - 其他技能：只放 summary，供模型需要时再去读取正文
//
// 这样做的原因是避免把所有技能全文都塞进 prompt，导致上下文过长且噪声过多。
type SkillCatalog interface {
	// GetAlwaysSkills 返回总是应当直接注入上下文的技能名列表。
	GetAlwaysSkills() ([]string, error)

	// LoadSkillsForContext 读取给定技能名的完整正文。
	LoadSkillsForContext(names []string) (string, error)

	// BuildSummary 生成可供模型“按需发现”的技能摘要。
	//
	// 返回：
	//  - string: 技能摘要
	//  - error: 生成失败时返回错误
	BuildSummary() (string, error)
}

// Builder 负责组装 system prompt 和一轮模型调用需要的消息序列。
//
// 为什么把它设计成“只读、无缓存”的小对象：
// - prompt 组装最重要的是可预测，而不是过早优化
// - workspace 文件是用户可编辑资产，过早缓存反而更容易制造“为什么没生效”的困惑
type Builder struct {
	workspacePath string
	bootstrap     BootstrapReader
	memory        MemoryReader
	skills        SkillCatalog
}

// NewPromptBuilder 使用显式依赖创建 prompt builder。
//
// 参数：
// - workspacePath: 当前 workspace 路径
// - bootstrap: bootstrap 文档读取器
// - memory: memory 读取器
// - skills: 技能目录读取器
//
// 返回：
// - *Builder: 可重复使用的 prompt builder
//
// 这种构造方式可以让测试只替换自己关心的依赖，降低 prompt 相关测试的搭建成本。
func NewPromptBuilder(workspacePath string, bootstrap BootstrapReader, memory MemoryReader, skills SkillCatalog) *Builder {
	workspacePath = strings.TrimSpace(workspacePath)

	return &Builder{
		workspacePath: workspacePath,
		bootstrap:     bootstrap,
		memory:        memory,
		skills:        skills,
	}
}

// BuildMessages 生成一次模型调用需要的完整消息列表。
//
// 参数：
// - history: 当前 session 历史
// - currentMessage: 本轮用户输入
// - skillNames: 当前轮显式选中的技能名；当前实现保留参数位点，但尚未真正使用
//
// 返回：
// - []map[string]any: provider 可直接消费的消息序列
//
// 为什么这里总是“system -> history -> current user”这个顺序：
// - system prompt 需要先出现，保证工作区规则优先于历史内容
// - 历史消息次之，保证对话连续性
// - 当前输入最后追加，保证模型把这一轮问题当作最新上下文
func (b *Builder) BuildMessages(history []*model.Message, currentMessage string, skillNames []string) []map[string]any {
	messages := make([]map[string]any, 0, len(history)+2)
	messages = append(messages, map[string]any{
		"role":    model.RoleSystem,
		"content": b.BuildSystemPrompt(skillNames),
	})
	messages = append(messages, toLLMMessages(history)...)
	messages = append(messages, map[string]any{
		"role":    model.RoleUser,
		"content": currentMessage,
	})
	return messages
}

// AddToolResult 把工具执行结果追加到 transcript 中。
//
// 这里做成单独 helper 的原因是：
// tool result 的消息形状属于 provider 交互协议，不适合散落在 service 里到处手拼。
func (b *Builder) AddToolResult(messages []map[string]any, toolCallID string, toolName string, result string) []map[string]any {
	return append(messages, map[string]any{
		"role":         model.RoleTool,
		"tool_call_id": toolCallID,
		"name":         toolName,
		"content":      result,
	})
}

// AddAssistantMessage 把 assistant 的正文或 tool-call 痕迹追加到 transcript 中。
//
// 这里允许 content 为空，是因为某些 provider 的工具调用响应可能只返回 tool_calls，
// 没有自然语言正文。
func (b *Builder) AddAssistantMessage(messages []map[string]any, content string, toolCalls []map[string]any) []map[string]any {
	msg := map[string]any{"role": model.RoleAssistant}
	if content != "" {
		msg["content"] = content
	}
	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
	}
	return append(messages, msg)
}

// BuildSystemPrompt 按固定顺序组装 system prompt。
//
// 参数：
// - skillNames: 当前轮显式选中的技能名；当前实现暂未真正使用
//
// 返回：
// - string: 完整 system prompt
//
// 当前顺序是刻意固定的：
// 1. identity
// 2. bootstrap docs
// 3. memory
// 4. always skills
// 5. skills summary
//
// 这样做的原因是：
// - identity 和 bootstrap docs 是最显式的规则，应当优先出现
// - memory 提供长期上下文，但不应压过明确规则
// - skills 更像能力扩展，放在后面更合理
func (b *Builder) BuildSystemPrompt(skillNames []string) string {
	// 当前版本还没有真正的“按需激活 skill 正文”逻辑。
	// 这里先保留参数，避免后续扩展时再修改上层 service 接口。
	//
	// TODO: 当 runtime 能根据模型/用户选择 skill 时，在这里接入 skillNames 的显式加载。
	_ = skillNames

	parts := []string{b.renderIdentity()}

	if docs := b.collectBootstrapDocs(); docs != "" {
		parts = append(parts, docs)
	}

	if b.memory != nil {
		if memoryContext := b.memory.BuildContext(); memoryContext != "" {
			parts = append(parts, fmt.Sprintf("## Memory\n%s", memoryContext))
		}
	}

	if b.skills != nil {
		alwaysSkills, err := b.skills.GetAlwaysSkills()
		if err == nil && len(alwaysSkills) > 0 {
			if activeSkills, loadErr := b.skills.LoadSkillsForContext(alwaysSkills); loadErr == nil && activeSkills != "" {
				parts = append(parts, fmt.Sprintf("# Active Skills\n\n%s", activeSkills))
			}
		}

		if summary, err := b.skills.BuildSummary(); err == nil && summary != "" {
			parts = append(parts, fmt.Sprintf(`# Skills

The following skills extend your capabilities. To use a skill, read its SKILL.md file using the read_file tool.
Skills with available="false" need dependencies installed first - you can try installing them with apt/brew.

%s`, summary))
		}
	}

	return strings.Join(parts, "\n\n--\n\n")
}

// collectBootstrapDocs 读取并按稳定顺序拼接 bootstrap 文档。
//
// 返回：
// - string: 拼接后的 bootstrap 区块；没有可用内容时返回空串
//
// 这里采用 soft-fail 策略：
// - 如果 reader 报错，但没有任何成功读取内容，就直接返回空串
// - 不因为一个缺失或损坏的 workspace 文件让整轮对话失败
//
// 这样更符合 agent 场景：workspace 文档是用户会手改的资产，系统应尽量继续工作。
func (b *Builder) collectBootstrapDocs() string {
	if b.workspacePath == "" || b.bootstrap == nil {
		return ""
	}

	loaded, err := b.bootstrap.Load(defaultBootstrapFiles)
	if err != nil && len(loaded) == 0 {
		return ""
	}

	parts := make([]string, 0, len(defaultBootstrapFiles))
	for _, name := range defaultBootstrapFiles {
		content := strings.TrimSpace(loaded[name])
		if content == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("## %s\n\n%s", name, content))
	}
	return strings.Join(parts, "\n\n")
}

// identityTemplate 返回基础 identity 模板。
//
// 这里保留了 workspace 路径、memory 文件位置和 message tool 使用约束，
// 原因是当前 tinybot 的行为高度依赖“模型知道自己该去哪里读文件、什么时候直接回复、什么时候走 message tool”。
func (b *Builder) identityTemplate() string {
	return `# tinybot

You are tinybot, a helpful AI assistant. You have access to tools that allow you to:
- Read, write, and edit files
- Execute shell commands
- Search the web and fetch web pages
- Send messages to users on chat channels

## Current Time
{now}

## Workspace
Your workspace is at: {workspace_path}
- Memory files: {workspace_path}/memory/MEMORY.md
- Daily notes: {workspace_path}/memory/YYYY-MM-DD.md
- Custom skills: {workspace_path}/skills/{skill-name}/SKILL.md

IMPORTANT: When responding to direct questions or conversations, reply directly with your text response.
Only use the 'message' tool when you need to send a message to a specific chat channel (like WhatsApp).
For normal conversation, just respond with text - do not call the message tool.

Always be helpful, accurate, and concise. When using tools, explain what you are doing.
When remembering something, write to {workspace_path}/memory/MEMORY.md`
}

// renderIdentity 把 identity 模板渲染成当前轮真实文本。
//
// 这里把当前时间直接注入 prompt，是为了让模型具备最基本的时间感知，
// 尤其在 memory、计划、日志类任务里更符合用户预期。
func (b *Builder) renderIdentity() string {
	path := b.workspacePath
	if path == "" {
		path = "."
	}
	return strings.NewReplacer(
		"{now}", time.Now().Format("2006-01-02 15:04:05"),
		"{workspace_path}", path,
	).Replace(b.identityTemplate())
}
