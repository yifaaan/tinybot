# nanobot Go rewrite plan

## 1. 当前 Go 代码库已经实现的部分

当前仓库已经有一条可运行的 MVP 闭环：

1. `cmd/tinybot/` 提供 CLI 入口，支持 `help`、`onboard`、`status`、`gateway`、`cron` 和直接聊天。
2. `internal/app/` 负责启动期装配：加载配置、创建 provider、创建 session repo、注册工具、构造 chat use case。
3. `internal/domain/model/` 已经定义了比较稳定的核心模型：
   - `InboundMessage` / `OutboundMessage`
   - `Session` / `Message`
   - `CronJob` / `CronSchedule`
   - `LLMResponse` / `ToolCall`
4. `internal/ports/` 已经抽出了主要边界接口：
   - `LLMClient`
   - `ToolRegistry`
   - `MessageBus`
   - `Channel`
   - `CronRepository`
   - `SessionRepository`
5. `internal/usecase/chat/` 已经实现核心 agent turn：
   - 获取 session
   - 构造 prompt 和历史消息
   - 调用 LLM
   - 执行工具调用
   - 保存 session
   - 返回回复
6. `internal/usecase/agent/` 已经实现了 bus 消费和回复发布循环。
7. `internal/usecase/cron/` 和 `internal/usecase/heartbeat/` 已经实现后台轮询服务雏形。
8. `internal/adapters/` 已经有主要外部适配器：
   - `provider/qwen.go`
   - `repository/file_session_repo.go`
   - `repository/file_cron_repo.go`
   - `bus/memory_bus.go`
   - `channel/console_channel.go`
   - `tool/*`
   - `workspace/*`

一句话总结：当前 Go 版本不是“从零开始”，而是“功能闭环已经有了，但目录边界和接口颗粒度还没有整理成最终形态”。

## 2. nanobot 原始模块分析

下面按“职责 / 输入输出 / 状态 / 副作用 / 当前 Go 对应关系”来分析原项目。

### 2.1 `nanobot/agent/loop.py`

**职责**

- 作为核心 agent turn processor。
- 从 bus 接收消息。
- 使用 `ContextBuilder` 组装系统提示词和历史消息。
- 调用 LLM provider。
- 处理 tool calling 循环。
- 保存 session。
- 将结果发布到 outbound bus。

**输入**

- `InboundMessage`
- provider
- workspace path
- tool registry
- session manager
- max iterations

**输出**

- `OutboundMessage`
- 更新后的 session 文件

**状态**

- `self._running`
- `SessionManager` 中的缓存 session
- 工具注册表

**副作用**

- 读写 session JSONL 文件
- 调用 LLM API
- 调用文件、shell、web、message 等工具
- 发布 outbound 消息

**当前 Go 对应**

- `internal/usecase/chat/usecase.go`
- `internal/usecase/agent/loop.go`
- `internal/adapters/tool/*`
- `internal/adapters/repository/file_session_repo.go`

**保持兼容时要注意**

- direct chat 和 bus chat 最终都应复用同一条 turn 处理逻辑。
- tool calling 要保留“多轮直到无工具调用”为止的行为。
- 工具失败应转换成可返回给模型的文本，而不是直接 panic 或中断整个进程。

### 2.2 `nanobot/agent/context.py`

**职责**

- 构造 system prompt。
- 读取 bootstrap 文件：`AGENTS.md`、`SOUL.md`、`USER.md`、`TOOLS.md`、`IDENTITY.md`。
- 拼接 memory 上下文。
- 拼接 skills 的 always-loaded 内容和技能摘要。
- 把历史消息和当前用户输入组合成 LLM 请求。

**输入**

- workspace path
- 历史消息
- 当前用户消息
- 可选技能名列表

**输出**

- `[]message`，用于发送给 LLM

**状态**

- `MemoryStore`
- `SkillsLoader`

**副作用**

- 读取 workspace 文件
- 读取 memory 文件
- 读取 skills 元数据和技能正文

**当前 Go 对应**

- `internal/usecase/chat/context_builder.go`
- `internal/adapters/workspace/memory_store.go`
- `internal/adapters/workspace/skills_loader.go`

**保持兼容时要注意**

- 当前 Go 版本没有把 `IDENTITY.md` 纳入 bootstrap 文件，这和原版有差异。
- 当前 Go 版本已经支持 memory 和 skills summary，但还没有把它们抽象成 service 层依赖。

### 2.3 `nanobot/agent/memory.py`

**职责**

- 管理长期记忆 `memory/MEMORY.md`
- 管理当天记忆 `memory/YYYY-MM-DD.md`
- 为 ContextBuilder 提供 memory context

**输入**

- workspace path
- 读写的文本内容

**输出**

- 记忆内容文本
- 记忆文件路径

**状态**

- `memory_dir`
- `memory_file`

**副作用**

- 读写磁盘文件

**当前 Go 对应**

- `internal/adapters/workspace/memory_store.go`

**保持兼容时要注意**

- 这是一个很适合优先稳定的模块，因为边界清晰、依赖简单、易测。

### 2.4 `nanobot/agent/skills.py`

**职责**

- 发现 workspace skill 和 builtin skill。
- 加载技能正文。
- 解析 frontmatter 元数据。
- 构建技能摘要，支持 progressive loading。
- 检查依赖是否满足。

**输入**

- workspace path
- builtin skills path
- 环境变量和本机命令可用性

**输出**

- skill 列表
- skill 正文
- skills summary

**状态**

- workspace skill 根目录
- builtin skill 根目录

**副作用**

- 扫描文件系统
- 读取技能文件
- 读取环境变量
- 检查二进制命令是否存在

**当前 Go 对应**

- `internal/adapters/workspace/skills_loader.go`
- `internal/adapters/workspace/skill_meta.go`

**保持兼容时要注意**

- 技能系统最好保持“摘要 + 按需 read_file 全量加载”的模式，不要一次性把所有技能塞进 prompt。

### 2.5 `nanobot/session/manager.py`

**职责**

- 获取或创建 session。
- 维护内存缓存。
- 持久化 JSONL session 文件。
- 列出和删除 session。

**输入**

- session key
- session message 列表

**输出**

- `Session`
- session metadata 列表

**状态**

- session cache

**副作用**

- 读写 session JSONL 文件

**当前 Go 对应**

- `internal/adapters/repository/file_session_repo.go`
- `internal/domain/model/session.go`

**保持兼容时要注意**

- 当前 Go 版本在 `Session.GetHistory()` 中处理了 `LastConsolidated` 和 orphan tool chain，这比原版更强。
- 但当前 `SessionRepository` 还没有统一携带 `context.Context`。

### 2.6 `nanobot/cron/service.py`

**职责**

- 管理 scheduled jobs。
- 从磁盘加载和保存 cron store。
- 计算下一次执行时间。
- 启动定时器并执行 due job。
- 支持 one-shot、interval、cron expression。
- 支持可选 deliver 到 channel。

**输入**

- store path
- job 定义
- on-job callback

**输出**

- 更新后的 cron store
- job 执行状态

**状态**

- `_store`
- `_timer_task`
- `_running`

**副作用**

- 读写 cron store JSON 文件
- 触发 agent turn
- 可选发布消息到 channel

**当前 Go 对应**

- `internal/usecase/cron/service.go`
- `internal/adapters/repository/file_cron_repo.go`
- `cmd/tinybot/run.go` 的 `cron` 子命令

**保持兼容时要注意**

- 当前 Go 版本只支持 `every`，原版支持 `every`、`cron`、`at`。
- 当前 Go 版本的 `TriggerOnce()` 会扫全量 job，但返回值信息较少。
- 当前 Go 版本还没有 `deliver`、`channel`、`to` 这些字段行为。

### 2.7 `nanobot/heartbeat/service.py`

**职责**

- 周期性读取 `HEARTBEAT.md`
- 判断是否存在 actionable content
- 通过 agent turn 执行心跳任务

**输入**

- workspace path
- on-heartbeat callback
- interval
- enabled

**输出**

- agent 回复文本

**状态**

- `_running`
- `_task`

**副作用**

- 读取 `HEARTBEAT.md`
- 触发 agent turn

**当前 Go 对应**

- `internal/usecase/heartbeat/service.go`

**保持兼容时要注意**

- 当前 Go 版本已经保留了核心行为。
- 下一步可以补日志、trace 和 request id。

### 2.8 `nanobot/channels/manager.py`

**职责**

- 初始化 channel 适配器。
- 启动 channel。
- 分发 outbound message 到对应 channel。

**输入**

- config
- message bus

**输出**

- 启动中的 channel 集合

**状态**

- channel registry
- outbound dispatch task

**副作用**

- 启动外部 channel SDK
- 发送消息到 Telegram / WhatsApp 等

**当前 Go 对应**

- `internal/adapters/channel/channal_manager.go`
- `internal/adapters/channel/console_channel.go`

**保持兼容时要注意**

- 当前 Go 版本目前只有 console channel 雏形，transport 层后续可继续补 Telegram / WhatsApp。

### 2.9 `nanobot/providers/base.py`

**职责**

- 约束 provider 能力边界：
  - chat completion
  - default model
- 把 provider 的协议差异挡在 domain/service 外部

**输入**

- messages
- tools
- model
- max tokens
- temperature

**输出**

- `LLMResponse`

**状态**

- provider client
- 默认 model

**副作用**

- 网络请求外部模型 API

**当前 Go 对应**

- `internal/ports/llm.go`
- `internal/adapters/provider/qwen.go`

**保持兼容时要注意**

- Go 版本已经有 provider 接口，但当前只实现了 Qwen。
- 将来要支持更多 provider 时，最好把 message/tool schema translator 再单独整理。

## 3. 目前与原版的主要差异

这些差异很重要，因为它们直接影响“保持现有行为一致”要做到什么程度。

### 已经对齐的部分

- 有 direct chat 和 gateway 两种入口。
- 有 session 持久化。
- 有 prompt builder、memory、skills summary。
- 有 tool registry 和 tool calling loop。
- 有 heartbeat 和 cron 服务原型。

### 还没完全对齐的部分

1. **Provider 能力**
   - 原版支持更通用的 provider 抽象和 OpenRouter 风格配置。
   - 当前 Go 版本主要绑定 Qwen。

2. **Cron 能力**
   - 原版支持 `every`、`cron`、`at`、`deliver`。
   - 当前 Go 版本只实现 `every`。

3. **Channel 能力**
   - 原版支持 Telegram / WhatsApp。
   - 当前 Go 版本当前只完成 console channel 主流程。

4. **Session 记录内容**
   - 原版 Python 只保存用户消息和最终 assistant 回复。
   - 当前 Go 版本会把 assistant tool calls 和 tool result 也保存进 session trace。
   - 这不是坏事，但它已经是一个“可观察行为差异”。

5. **Workspace bootstrap**
   - 原版会读取 `IDENTITY.md`。
   - 当前 Go 版本默认 bootstrap 文件里没有这一项。

6. **目录结构**
   - 当前 Go 使用 `internal/usecase` 和 `internal/adapters`。
   - 你的目标结构希望收敛到 `domain / service / repository / transport`。

## 4. 建议的目标包结构

下面是保持简单、贴近当前实现、又符合项目目标的一版目标结构：

```text
cmd/
  tinybot/
    main.go
    run.go

internal/
  domain/
    model/
      message.go
      session.go
      cron.go
      llm.go
    errors/
      errors.go

  service/
    chat/
      service.go
      prompt_builder.go
    runtime/
      loop.go
    cron/
      service.go
    heartbeat/
      service.go
    onboarding/
      service.go
    status/
      service.go

  repository/
    provider/
      qwen.go
    session/
      file_repository.go
    cron/
      file_repository.go
    workspace/
      memory_store.go
      skills_loader.go
      bootstrap_reader.go
    bus/
      memory_bus.go

  transport/
    cli/
      run.go
      cron.go
    channel/
      console.go
      manager.go

pkg/
  fsx/
  timex/
```

### 为什么这样拆

- `domain/` 只放稳定的数据结构和业务规则。
- `service/` 只放业务流程编排，不直接 new 具体实现。
- `repository/` 放持久化和 provider adapter，符合本项目的架构约束。
- `transport/` 放 CLI 和聊天通道。
- `cmd/` 只保留最薄的一层启动代码。

## 5. 推荐优先实现顺序

对于 Go 初学者，我建议**不要先重命名所有目录**，而是先按下面顺序增量重构：

### 第一步：冻结领域模型和 service 边界

先做这些小改动：

1. 给 `SessionRepository` 增加 `context.Context`
2. 给 chat 核心 service 定义更明确的依赖接口
3. 把 `ContextBuilder` 依赖改成接口而不是直接依赖具体 workspace adapter

**为什么先做这个**

- 这是整个系统的“稳定接缝”。
- 一旦 service 边界清楚，后面移动目录不会乱。

### 第二步：把 `internal/usecase/chat` 整理成 `internal/service/chat`

目标不是重写逻辑，而是整理责任边界：

- `Service` 负责 turn orchestration
- `PromptBuilder` 负责 prompt 组装
- `ToolRunner` 负责工具定义和执行
- `SessionRepository` 负责持久化

### 第三步：迁移 repository adapter

按风险从低到高迁移：

1. `workspace/memory_store`
2. `workspace/skills_loader`
3. `session/file_repository`
4. `cron/file_repository`
5. `provider/qwen`

### 第四步：迁移 runtime 和 transport

- `usecase/agent/loop` -> `service/runtime/loop`
- `adapters/channel/*` -> `transport/channel/*`
- `cmd/tinybot/run.go` 里的 cron 子命令业务逻辑下沉到 `service/cron`

### 第五步：补齐原版缺口

- `cron` 支持 `at` / `cron`
- `deliver` 到 channel
- `IDENTITY.md`
- Telegram / WhatsApp transport
- 更完整的 provider abstraction

## 6. 代码骨架

下面的骨架故意只给出最小结构和 TODO，适合你自己继续补实现。

### 6.1 `internal/service/chat/service.go`

```go
package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tinybot/internal/domain/model"
)

// SessionRepository 定义 chat service 对 session 持久化的最小依赖。
// 先收窄接口，再由 file repository 去实现它。
type SessionRepository interface {
	GetOrCreate(ctx context.Context, key string) (*model.Session, error)
	Save(ctx context.Context, session *model.Session) error
}

// ModelClient 是模型调用边界。
// service 层不关心 Qwen/OpenAI/OpenRouter 的具体协议。
type ModelClient interface {
	Chat(
		ctx context.Context,
		messages []map[string]any,
		tools []map[string]any,
		maxTokens int,
		temperature float32,
	) (model.LLMResponse, error)
}

// ToolRunner 抽象工具注册和执行。
type ToolRunner interface {
	GetDefinitions() []map[string]any
	Execute(ctx context.Context, name string, params map[string]any) (string, error)
}

// PromptBuilder 负责把领域对象翻译成模型输入消息。
type PromptBuilder interface {
	BuildMessages(history []*model.Message, current string, skillNames []string) []map[string]any
	AddAssistantMessage(messages []map[string]any, content string, toolCalls []map[string]any) []map[string]any
	AddToolResult(messages []map[string]any, toolCallID string, toolName string, result string) []map[string]any
}

// Service 负责单次 agent turn 的完整编排。
// 它不直接依赖文件系统、网络或具体 provider 实现。
type Service struct {
	sessions      SessionRepository
	model         ModelClient
	tools         ToolRunner
	prompts       PromptBuilder
	maxIterations int
	maxTokens     int
}

func NewService(
	sessions SessionRepository,
	model ModelClient,
	tools ToolRunner,
	prompts PromptBuilder,
	maxIterations int,
) (*Service, error) {
	if sessions == nil {
		return nil, fmt.Errorf("chat service: sessions is required")
	}
	if model == nil {
		return nil, fmt.Errorf("chat service: model is required")
	}
	if tools == nil {
		return nil, fmt.Errorf("chat service: tools is required")
	}
	if prompts == nil {
		return nil, fmt.Errorf("chat service: prompts is required")
	}
	if maxIterations <= 0 {
		maxIterations = 8
	}

	return &Service{
		sessions:      sessions,
		model:         model,
		tools:         tools,
		prompts:       prompts,
		maxIterations: maxIterations,
		maxTokens:     8192,
	}, nil
}

// ProcessMessage 负责:
// 1. 读取 session
// 2. 构造 LLM 输入
// 3. 执行 tool loop
// 4. 保存 trace
// 5. 返回 outbound message
func (s *Service) ProcessMessage(ctx context.Context, in model.InboundMessage) (model.OutboundMessage, error) {
	// TODO: 迁移现有 internal/usecase/chat/usecase.go 的主流程
	// TODO: 明确 trace 持久化策略，决定是否兼容保留 tool messages
	// TODO: 所有 repository I/O 都改为显式传递 ctx
	return model.OutboundMessage{}, nil
}

// ProcessDirect 是给 CLI、heartbeat、cron 复用的简化入口。
func (s *Service) ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("chat service: content is empty")
	}

	in := model.InboundMessage{
		ID:       fmt.Sprintf("direct-%d", time.Now().UnixNano()),
		Channel:  model.ChannelCLI,
		SenderID: "user",
		ChatID:   "direct",
		Content:  content,
	}

	if strings.TrimSpace(sessionKey) != "" {
		in.SessionKeyOverride = &sessionKey
	}

	out, err := s.ProcessMessage(ctx, in)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out.Content), nil
}
```

### 6.2 `internal/service/runtime/loop.go`

```go
package runtime

import (
	"context"
	"fmt"
	"strings"

	"tinybot/internal/domain/model"
)

type Bus interface {
	ConsumeInbound(ctx context.Context) (model.InboundMessage, error)
	PublishOutbound(ctx context.Context, msg model.OutboundMessage) error
}

type MessageProcessor interface {
	ProcessMessage(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error)
}

// Loop 负责运行时消息循环，不关心 session、prompt、LLM 细节。
type Loop struct {
	bus       Bus
	processor MessageProcessor
}

func NewLoop(bus Bus, processor MessageProcessor) (*Loop, error) {
	if bus == nil {
		return nil, fmt.Errorf("runtime loop: bus is required")
	}
	if processor == nil {
		return nil, fmt.Errorf("runtime loop: processor is required")
	}
	return &Loop{bus: bus, processor: processor}, nil
}

func (l *Loop) Run(ctx context.Context) error {
	for {
		in, err := l.bus.ConsumeInbound(ctx)
		if err != nil {
			return err
		}

		out, err := l.processor.ProcessMessage(ctx, in)
		if err != nil {
			fallback := model.OutboundMessage{
				Channel: in.Channel,
				ChatID:  in.ChatID,
				ReplyTo: in.ID,
				Content: fmt.Sprintf("Sorry, I encountered an error: %v", err),
			}
			if pubErr := l.bus.PublishOutbound(ctx, fallback); pubErr != nil {
				return pubErr
			}
			continue
		}

		if strings.TrimSpace(out.Content) == "" {
			continue
		}
		if err := l.bus.PublishOutbound(ctx, out); err != nil {
			return err
		}
	}
}
```

### 6.3 `internal/repository/session/file_repository.go`

```go
package session

import (
	"context"
	"fmt"
	"path/filepath"

	"tinybot/internal/domain/model"
)

// FileRepository 负责 JSONL session 持久化。
// 它属于 repository 层，因为它是显式的磁盘适配器。
type FileRepository struct {
	workspace  string
	sessionDir string
	cache      map[string]*model.Session
}

func NewFileRepository(workspace string) *FileRepository {
	return &FileRepository{
		workspace:  workspace,
		sessionDir: filepath.Join(workspace, "sessions"),
		cache:      make(map[string]*model.Session),
	}
}

func (r *FileRepository) GetOrCreate(ctx context.Context, key string) (*model.Session, error) {
	// TODO: 迁移当前 file_session_repo.go 的缓存和加载逻辑
	// TODO: 为未来 tracing/logging 预留 ctx
	return nil, fmt.Errorf("TODO")
}

func (r *FileRepository) Save(ctx context.Context, session *model.Session) error {
	// TODO: 迁移当前 file_session_repo.go 的 JSONL 落盘逻辑
	return fmt.Errorf("TODO")
}
```

### 6.4 `internal/service/chat/prompt_builder.go`

```go
package chat

import (
	"fmt"
	"strings"

	"tinybot/internal/domain/model"
)

// BootstrapReader 读取 AGENTS.md、SOUL.md、USER.md 等 bootstrap 文件。
type BootstrapReader interface {
	Load() (map[string]string, error)
}

// MemoryReader 读取长期记忆和当天笔记。
type MemoryReader interface {
	BuildContext() string
}

// SkillCatalog 提供 always-loaded skills 和 skills summary。
type SkillCatalog interface {
	GetAlwaysSkills() []string
	LoadSkillsForContext(skillNames []string) (string, error)
	BuildSummary() (string, error)
}

// Builder 负责 system prompt 和消息拼装。
type Builder struct {
	workspace  string
	bootstrap  BootstrapReader
	memory     MemoryReader
	skills     SkillCatalog
}

func NewBuilder(workspace string, bootstrap BootstrapReader, memory MemoryReader, skills SkillCatalog) *Builder {
	return &Builder{
		workspace: workspace,
		bootstrap: bootstrap,
		memory:    memory,
		skills:    skills,
	}
}

func (b *Builder) BuildMessages(history []*model.Message, current string, skillNames []string) []map[string]any {
	// TODO: 迁移现有 ContextBuilder.BuildMessages
	return nil
}

func (b *Builder) AddAssistantMessage(messages []map[string]any, content string, toolCalls []map[string]any) []map[string]any {
	msg := map[string]any{"role": model.RoleAssistant}
	if strings.TrimSpace(content) != "" {
		msg["content"] = content
	}
	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
	}
	return append(messages, msg)
}

func (b *Builder) AddToolResult(messages []map[string]any, toolCallID string, toolName string, result string) []map[string]any {
	return append(messages, map[string]any{
		"role":         model.RoleTool,
		"tool_call_id": toolCallID,
		"name":         toolName,
		"content":      result,
	})
}

func (b *Builder) BuildSystemPrompt(skillNames []string) (string, error) {
	// TODO: 迁移现有 BuildSystemPrompt
	// TODO: 把 IDENTITY.md 纳入 bootstrap reader
	return fmt.Sprintf("workspace=%s", b.workspace), nil
}
```

## 7. 适合你的分步实现路线

下面是一条更适合初学者的实现路线，每一步都尽量是“小而能跑”的。

### Milestone 1: service 接缝固定

**目标**

- 先不搬目录，先把接口整理出来。

**你要改的地方**

- 给 `internal/ports/session.go` 增加 `context.Context`
- 给 `internal/usecase/chat/usecase.go` 增加更窄的依赖接口
- 给 `ContextBuilder` 增加可注入依赖

**你会学到**

- Go 接口应该怎么围绕“使用方”定义
- 为什么依赖注入通常放在 service 边界，而不是 domain

### Milestone 2: prompt builder 整理

**目标**

- 先把最容易稳定的一块抽出来：bootstrap + memory + skills。

**建议实现**

1. 新建 `BootstrapReader` 接口
2. 让现有 workspace adapter 去实现它
3. 给 `BuildSystemPrompt()` 写 table-driven tests

**建议测试点**

- 没有 workspace 时是否仍能生成最小 system prompt
- 有 `AGENTS.md`、`SOUL.md` 时是否按顺序拼接
- memory 为空和非空时的输出差异
- always-loaded skills 和 summary 的拼接顺序

### Milestone 3: session repository 整理

**目标**

- 让所有 session I/O 都走 `context.Context`
- 固化 JSONL 文件格式

**建议测试点**

- `GetOrCreate()` 在缓存命中和磁盘命中时行为一致
- `Save()` 后再 `Load()` 能 round-trip
- `GetHistory()` 会裁剪过长历史
- tool trace 是否保留，作为显式兼容决策写进测试

### Milestone 4: chat service 整理

**目标**

- 把 turn orchestration 稳定下来。

**建议测试点**

- 空消息报错
- 无 tool call 时直接返回
- 有多轮 tool call 时按顺序执行
- 工具执行失败时是否把错误文本回注给模型
- session 是否被保存
- direct chat 是否复用同一条主流程

### Milestone 5: cron 和 heartbeat 补齐

**目标**

- 保留当前能跑的行为，同时逐步靠近原版功能。

**先补哪些**

1. `cron` 的 `at`
2. `cron` 的 `cron expression`
3. `deliver` 到 channel
4. heartbeat 的日志和 trace

## 8. 测试策略

建议把测试分成四类：

### 领域模型测试

- `Session.GetHistory()`
- `CronJob.Validate()`
- `CronJob.ComputeNextRun()`
- `InboundMessage.SessionKey()`

### service 测试

- `chat.Service.ProcessMessage()`
- `chat.Builder.BuildSystemPrompt()`
- `runtime.Loop.Run()`
- `cron.Service.TriggerOnce()`
- `heartbeat.Service.TriggerOnce()`

### repository 测试

- file session repo round-trip
- file cron repo round-trip
- workspace memory/skills loader

### transport 测试

- CLI 命令分发
- console channel 收发

推荐全部用 table-driven tests，把“兼容行为”写成用例名字，例如：

- `keeps_tool_results_in_session_trace`
- `skips_empty_heartbeat_file`
- `runs_due_every_job`
- `loads_workspace_skill_before_builtin_skill`

## 9. 这次重写最值得先做的一个小任务

如果你现在只想做一个最合适的下一步，我建议：

**先把 `ContextBuilder` 的依赖接口抽出来，并给 `BuildSystemPrompt()` 补 table-driven tests。**

原因：

- 它直接连接 nanobot 的 bootstrap / memory / skills 三块原始设计。
- 它比 provider、channel、cron 更容易改，不容易把系统改坏。
- 做完这一步，你会真正掌握 Go 里的“面向接口编排 + 可测试设计”。

## 10. 本轮验收标准

本轮先不追求大规模重构，先追求下面这些结果：

- 你能说清楚原版 nanobot 各模块的职责边界。
- 你能说清楚当前 Go 版本已经完成了哪些能力、还缺哪些能力。
- 你有一份可执行的目标包结构和代码骨架。
- 你有一条适合初学者逐步落地的实现路线。
- 后续每一步都可以通过 table-driven tests 验证兼容性。
