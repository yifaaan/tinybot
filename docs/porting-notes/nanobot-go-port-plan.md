# nanobot Go Port Plan

This note captures:

- what the current Go rewrite already implements
- how the `nanobot/` reference project is organized
- which behaviors should stay compatible during the port
- a target `domain / service / repository / transport` layout
- code skeletons with TODOs for the next incremental rewrite steps

It intentionally avoids changing existing business code so it does not interfere
with the user's local edits in `AGENTS.md` and `.tinybot/workspace/config.json`.

## 1. Current Go Rewrite Status

The current Go workspace is no longer an empty rewrite. It already has a working
vertical slice for direct chat and a partial long-running runtime.

### Already implemented

- `cmd/tinybot/`
  - CLI entrypoint
  - `help`, `onboard`, `status`, `gateway`, `cron`
  - direct chat path for `tinybot <message>`
- `internal/app/`
  - config loading
  - workspace bootstrap/onboarding
  - gateway assembly
- `internal/domain/model/`
  - `Message`, `Session`, `InboundMessage`, `OutboundMessage`
  - `LLMResponse`, `ToolCall`
  - `CronJob`, `CronSchedule`
- `internal/usecase/chat/`
  - main agent turn orchestration
  - tool-call loop
  - context builder hook
- `internal/usecase/agent/`
  - bus-driven runtime loop
- `internal/usecase/cron/`
  - periodic cron scan and execution
- `internal/usecase/heartbeat/`
  - heartbeat polling and trigger logic
- `internal/adapters/provider/`
  - Qwen-compatible LLM adapter
- `internal/adapters/repository/`
  - JSONL session persistence
  - file-based cron storage
- `internal/adapters/tool/`
  - `exec`, `read_file`, `write_file`, `edit_file`, `list_dir`
  - `web_search`, `web_fetch`, `message`
- `internal/adapters/workspace/`
  - memory store
  - skills loader
- `internal/adapters/bus/`
  - in-memory message bus
- `internal/adapters/channel/`
  - console channel
  - channel manager

### Main gaps relative to the requested target architecture

- `internal/ports/` is a broad shared package; some interfaces are still shaped
  around adapter details instead of service needs.
- `internal/usecase/chat/` mixes three concerns:
  - conversation orchestration
  - prompt/context assembly
  - trace persistence details
- `internal/adapters/bus/` and `internal/adapters/channel/` are runtime transport
  concerns, but the current folder names do not make that obvious.
- `internal/adapters/workspace/` has good beginnings, but the service boundary is
  still file-system flavored.
- external dependencies are mostly injected already, but some constructors still
  allocate concrete dependencies directly inside the composition root.

## 2. Reference `nanobot/` Module Analysis

The Python reference project is best understood as a small set of cooperating
subsystems. The goal of the port is to preserve observable behavior first and
improve internal structure second.

### 2.1 `agent/loop.py`

Responsibilities:

- receive inbound messages from the bus
- load or create a session
- build LLM messages from workspace context and history
- call the model
- execute tool calls until a final assistant answer is produced
- persist the conversation
- publish the outbound reply

Inputs:

- inbound chat message
- session history
- workspace files
- tool definitions and tool results
- LLM response stream (single-shot in current implementation)

Outputs:

- final assistant text
- updated session JSONL
- optional outbound channel message

State transitions:

1. idle
2. receive inbound message
3. get session
4. build prompt
5. call LLM
6. if tool calls exist: append assistant tool-call message -> execute tools -> append tool results -> repeat step 5
7. else: produce final answer
8. persist session
9. publish outbound reply

Side effects:

- file reads from workspace and session store
- file writes to session store
- network call to model provider
- subprocess / filesystem / network side effects triggered by tools

### 2.2 `agent/context.py`

Responsibilities:

- build the system prompt from identity + bootstrap docs + memory + skills
- append history and current user message
- append assistant tool-call messages and tool result messages

Inputs:

- workspace path
- bootstrap files: `AGENTS.md`, `SOUL.md`, `USER.md`, `TOOLS.md`, optional `IDENTITY.md`
- memory files
- skill metadata and skill content
- session history
- current user message

Outputs:

- `[]message` in model-facing format
- deterministic prompt ordering

State:

- no durable mutable state beyond references to workspace readers

Side effects:

- reads workspace files and skill files

### 2.3 `session/manager.py`

Responsibilities:

- cache sessions in memory
- load and save sessions in JSONL format
- expose session history for prompt building

Inputs:

- session key
- session messages

Outputs:

- in-memory session object
- JSONL session file

State:

- in-memory session cache
- durable JSONL session files

Side effects:

- file reads and writes

### 2.4 `agent/tools/*` and `agent/tools/registry.py`

Responsibilities:

- register tool definitions
- expose model-facing schemas
- execute tool calls by name

Inputs:

- tool name
- tool args
- current workspace and environment

Outputs:

- tool schema list
- tool execution result string

State:

- registry map of tool name -> tool implementation

Side effects:

- filesystem reads and writes
- subprocess execution
- network access for web tools
- channel delivery for message tool

### 2.5 `providers/litellm_provider.py`

Responsibilities:

- adapt external model providers to a single `chat()` contract
- translate messages and tool definitions
- normalize tool call outputs

Inputs:

- API credentials
- messages
- tool schemas
- model name and generation options

Outputs:

- normalized `LLMResponse`

Side effects:

- network access
- environment variable writes for provider SDK configuration

### 2.6 `config/*` and `cli/commands.py`

Responsibilities:

- load/save user config
- initialize workspace
- route CLI commands into services

Inputs:

- config file path
- CLI arguments
- workspace path

Outputs:

- runtime configuration
- created workspace files
- console output

Side effects:

- file reads and writes
- console I/O

### 2.7 `bus/*` and `channels/*`

Responsibilities:

- decouple inbound/outbound chat transport from the core agent
- route replies to the right channel adapter

Inputs:

- inbound messages from channel adapters
- outbound messages from the agent

Outputs:

- queued messages
- channel sends

State:

- in-memory queues
- registered channels / subscribers

Side effects:

- goroutines/tasks
- network access for chat channels
- stdin/stdout for console channel

### 2.8 `cron/service.py`

Responsibilities:

- store jobs
- compute next run times
- wake up when jobs are due
- execute jobs through the agent callback
- persist run status

Inputs:

- cron store file
- wall clock
- agent callback

Outputs:

- updated job state
- optional channel delivery

State transitions:

1. load jobs
2. compute next wake
3. sleep
4. execute due jobs
5. update `last_run`, `last_error`, `next_run`
6. persist store

Side effects:

- timer scheduling
- file reads and writes
- agent execution

### 2.9 `heartbeat/service.py`

Responsibilities:

- periodically check `HEARTBEAT.md`
- skip if it has no actionable content
- ask the agent to process the heartbeat prompt

Inputs:

- heartbeat file
- timer tick
- agent callback

Outputs:

- optional agent response

State transitions:

1. idle
2. wait for interval
3. read heartbeat file
4. skip or invoke agent

Side effects:

- timer scheduling
- file reads
- agent execution

## 3. Best Modules to Port First

For a Go beginner, the best order is:

1. Workspace context stack
   - `memory`
   - `skills`
   - `context builder`
   - Why first: deterministic, mostly file I/O, easy to test with temp dirs
2. Session persistence
   - Why second: stable data model, preserves conversation history behavior
3. Conversation turn service
   - Why third: this is the real core of the agent and depends on the first two
4. CLI bootstrap and config
   - Why fourth: makes the local tool easy to run repeatedly
5. Cron and heartbeat
   - Why fifth: introduces timers but still stays local and testable
6. Bus and console/runtime loop
   - Why sixth: adds concurrency and lifecycle management
7. External channels
   - Why last: highest coupling to third-party APIs and long-running runtime

The current Go rewrite has already covered much of steps 1-6, so the next real
value is not "start from zero" but "tighten boundaries and align behavior".

## 4. Target Go Package Layout

Use the requested layers directly and keep adapters thin:

```text
cmd/
  tinybot/
    main.go
    run.go

internal/
  domain/
    agent/
      message.go
      session.go
      tool.go
    schedule/
      job.go
    workspace/
      skill.go

  service/
    conversation/
      service.go
      prompt_builder.go
    runtime/
      loop.go
    scheduler/
      cron.go
      heartbeat.go

  repository/
    session/
      file_repository.go
    schedule/
      file_repository.go
    llm/
      qwen_client.go
    workspace/
      memory_store.go
      skill_catalog.go
      config_store.go

  transport/
    cli/
      run.go
    channel/
      console.go
      manager.go
    bus/
      memory_bus.go

pkg/
  clock/
  textutil/
```

### Current-to-target mapping

- `internal/usecase/chat` -> `internal/service/conversation`
- `internal/usecase/agent` -> `internal/service/runtime`
- `internal/usecase/cron` -> `internal/service/scheduler`
- `internal/usecase/heartbeat` -> `internal/service/scheduler`
- `internal/adapters/repository` -> `internal/repository/session` and `internal/repository/schedule`
- `internal/adapters/provider` -> `internal/repository/llm`
- `internal/adapters/workspace` -> `internal/repository/workspace`
- `internal/adapters/bus` -> `internal/transport/bus`
- `internal/adapters/channel` -> `internal/transport/channel`

## 5. Interface Design

Keep interfaces at service boundaries and make them reflect the service's actual
needs rather than the underlying file format.

### 5.1 Conversation service interfaces

```go
package conversation

import (
    "context"
    "time"

    "tinybot/internal/domain/agent"
)

// SessionStore hides JSONL or any future storage backend from the service.
type SessionStore interface {
    GetOrCreate(ctx context.Context, key string) (*agent.Session, error)
    Save(ctx context.Context, session *agent.Session) error
}

// ModelClient hides Qwen/OpenAI/OpenRouter specific SDK details.
type ModelClient interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResult, error)
}

// ToolCatalog exposes both tool definitions and execution.
type ToolCatalog interface {
    Definitions(ctx context.Context) []agent.ToolDefinition
    Execute(ctx context.Context, call agent.ToolCall) (agent.ToolResult, error)
}

// PromptBuilder owns prompt assembly and message list mutation rules.
type PromptBuilder interface {
    Build(ctx context.Context, session *agent.Session, current string, skills []string) ([]agent.PromptMessage, error)
    AppendAssistant(messages []agent.PromptMessage, content string, calls []agent.ToolCall) []agent.PromptMessage
    AppendToolResult(messages []agent.PromptMessage, result agent.ToolResult) []agent.PromptMessage
}

// Clock makes timestamps deterministic in tests.
type Clock interface {
    Now() time.Time
}
```

### 5.2 Runtime service interfaces

```go
package runtime

import (
    "context"

    "tinybot/internal/domain/agent"
)

type MessageProcessor interface {
    HandleInbound(ctx context.Context, msg agent.InboundMessage) (agent.OutboundMessage, error)
}

type Bus interface {
    PublishInbound(ctx context.Context, msg agent.InboundMessage) error
    ConsumeInbound(ctx context.Context) (agent.InboundMessage, error)
    PublishOutbound(ctx context.Context, msg agent.OutboundMessage) error
    ConsumeOutbound(ctx context.Context) (agent.OutboundMessage, error)
    Close() error
}
```

### 5.3 Scheduler service interfaces

```go
package scheduler

import (
    "context"

    "tinybot/internal/domain/schedule"
)

type JobRepository interface {
    List(ctx context.Context) ([]schedule.Job, error)
    Save(ctx context.Context, jobs []schedule.Job) error
}

type AgentTurner interface {
    ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error)
}
```

## 6. Recommended Domain Structs

The exported domain types should stay small, stable, and persistence-agnostic.

```go
package agent

import "time"

type Role string

const (
    RoleSystem    Role = "system"
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleTool      Role = "tool"
)

// PromptMessage is the model-facing message shape.
type PromptMessage struct {
    Role       Role
    Content    string
    Name       string
    ToolCallID string
    ToolCalls  []ToolCall
}

// Message is the persisted conversation message.
type Message struct {
    Role       Role
    Content    string
    Name       string
    ToolCallID string
    ToolCalls  []ToolCall
    CreatedAt  time.Time
}

type Session struct {
    Key              string
    Messages         []Message
    Metadata         map[string]any
    CreatedAt        time.Time
    UpdatedAt        time.Time
    LastConsolidated int
}

type ToolDefinition struct {
    Name        string
    Description string
    Parameters  []byte
}

type ToolCall struct {
    ID   string
    Name string
    Args map[string]any
}

type ToolResult struct {
    ToolCallID string
    Name       string
    Content    string
}

type InboundMessage struct {
    ID        string
    Channel   string
    SenderID  string
    ChatID    string
    Content   string
    SessionID string
}

type OutboundMessage struct {
    Channel   string
    ChatID    string
    ReplyTo   string
    Content   string
    Metadata  map[string]any
    CreatedAt time.Time
}
```

## 7. Code Skeletons With TODOs

These skeletons are intentionally incomplete. They are designed for guided
implementation practice, not for a one-shot full refactor.

### 7.1 `internal/service/conversation/service.go`

```go
package conversation

import (
    "context"
    "errors"
    "fmt"
    "strings"

    "tinybot/internal/domain/agent"
)

// Service handles one complete inbound -> assistant reply turn.
// It preserves observable behavior:
// - load or create session
// - build prompt
// - call model
// - execute tool calls until a final answer exists
// - persist trace
// - return outbound reply
type Service struct {
    sessions      SessionStore
    models        ModelClient
    prompts       PromptBuilder
    tools         ToolCatalog
    clock         Clock
    maxIterations int
    maxTokens     int
}

func New(
    sessions SessionStore,
    models ModelClient,
    prompts PromptBuilder,
    tools ToolCatalog,
    clock Clock,
    maxIterations int,
    maxTokens int,
) (*Service, error) {
    switch {
    case sessions == nil:
        return nil, errors.New("conversation service: session store is required")
    case models == nil:
        return nil, errors.New("conversation service: model client is required")
    case prompts == nil:
        return nil, errors.New("conversation service: prompt builder is required")
    case tools == nil:
        return nil, errors.New("conversation service: tool catalog is required")
    case clock == nil:
        return nil, errors.New("conversation service: clock is required")
    case maxIterations <= 0:
        return nil, errors.New("conversation service: maxIterations must be positive")
    }

    return &Service{
        sessions:      sessions,
        models:        models,
        prompts:       prompts,
        tools:         tools,
        clock:         clock,
        maxIterations: maxIterations,
        maxTokens:     maxTokens,
    }, nil
}

// HandleInbound executes one full agent turn.
func (s *Service) HandleInbound(ctx context.Context, msg agent.InboundMessage) (agent.OutboundMessage, error) {
    if strings.TrimSpace(msg.Content) == "" {
        return agent.OutboundMessage{}, errors.New("conversation service: empty inbound content")
    }

    session, err := s.sessions.GetOrCreate(ctx, sessionKey(msg))
    if err != nil {
        return agent.OutboundMessage{}, fmt.Errorf("conversation service load session: %w", err)
    }

    messages, err := s.prompts.Build(ctx, session, msg.Content, nil)
    if err != nil {
        return agent.OutboundMessage{}, fmt.Errorf("conversation service build prompt: %w", err)
    }

    // TODO: implement the tool-call loop.
    // 1. call model
    // 2. if no tool calls -> finalize answer
    // 3. else append assistant tool-call message
    // 4. execute each tool via ToolCatalog
    // 5. append tool results
    // 6. repeat until final answer or iteration limit

    // TODO: persist the full trace back into the session.

    return agent.OutboundMessage{
        Channel:   msg.Channel,
        ChatID:    msg.ChatID,
        ReplyTo:   msg.ID,
        Content:   "",
        Metadata:  map[string]any{"session_key": session.Key},
        CreatedAt: s.clock.Now(),
    }, nil
}

func sessionKey(msg agent.InboundMessage) string {
    if strings.TrimSpace(msg.SessionID) != "" {
        return msg.SessionID
    }
    return msg.Channel + ":" + msg.ChatID
}
```

### 7.2 `internal/service/conversation/prompt_builder.go`

```go
package conversation

import (
    "context"
    "fmt"
    "strings"

    "tinybot/internal/domain/agent"
)

// MemoryReader provides formatted memory context.
type MemoryReader interface {
    BuildContext(ctx context.Context) (string, error)
}

// SkillCatalog provides progressive skill loading.
type SkillCatalog interface {
    AlwaysSkills(ctx context.Context) ([]string, error)
    LoadForContext(ctx context.Context, names []string) (string, error)
    Summary(ctx context.Context) (string, error)
}

// BootstrapReader loads static workspace bootstrap documents.
type BootstrapReader interface {
    Load(ctx context.Context) ([]Document, error)
}

type Document struct {
    Name    string
    Content string
}

// Builder assembles the system prompt and LLM message list.
type Builder struct {
    bootstrap BootstrapReader
    memory    MemoryReader
    skills    SkillCatalog
    workspace string
}

func NewBuilder(workspace string, bootstrap BootstrapReader, memory MemoryReader, skills SkillCatalog) *Builder {
    return &Builder{
        bootstrap: bootstrap,
        memory:    memory,
        skills:    skills,
        workspace: workspace,
    }
}

func (b *Builder) Build(ctx context.Context, session *agent.Session, current string, selected []string) ([]agent.PromptMessage, error) {
    messages := []agent.PromptMessage{
        {
            Role:    agent.RoleSystem,
            Content: b.systemPrompt(ctx, selected),
        },
    }

    // TODO: append history from session.Messages.
    // TODO: append the current user message.

    return messages, nil
}

func (b *Builder) systemPrompt(ctx context.Context, selected []string) string {
    parts := []string{
        b.identitySection(),
    }

    // TODO: load bootstrap docs with soft-fail behavior.
    // TODO: load memory context with soft-fail behavior.
    // TODO: load always-on skill content with soft-fail behavior.
    // TODO: append skill summary for progressive loading.

    _ = ctx
    _ = selected
    return strings.Join(parts, "\n\n---\n\n")
}

func (b *Builder) identitySection() string {
    return fmt.Sprintf(`# tinybot

You are tinybot, a helpful AI assistant.

## Workspace
%s

Use tools carefully and keep context across turns.`, b.workspace)
}

func (b *Builder) AppendAssistant(messages []agent.PromptMessage, content string, calls []agent.ToolCall) []agent.PromptMessage {
    return append(messages, agent.PromptMessage{
        Role:      agent.RoleAssistant,
        Content:   content,
        ToolCalls: calls,
    })
}

func (b *Builder) AppendToolResult(messages []agent.PromptMessage, result agent.ToolResult) []agent.PromptMessage {
    return append(messages, agent.PromptMessage{
        Role:       agent.RoleTool,
        Name:       result.Name,
        ToolCallID: result.ToolCallID,
        Content:    result.Content,
    })
}
```

### 7.3 `internal/repository/session/file_repository.go`

```go
package session

import (
    "context"
    "fmt"
    "path/filepath"

    "tinybot/internal/domain/agent"
)

// FileRepository stores sessions as JSONL.
// The service should only depend on GetOrCreate/Save, not file paths.
type FileRepository struct {
    root string
    // TODO: add cache and optional mutex if concurrent access becomes required.
}

func New(root string) *FileRepository {
    return &FileRepository{root: root}
}

func (r *FileRepository) GetOrCreate(ctx context.Context, key string) (*agent.Session, error) {
    // TODO: 1. honor ctx cancellation
    // TODO: 2. load existing JSONL if present
    // TODO: 3. otherwise return a new session
    _ = ctx
    _ = key
    return nil, nil
}

func (r *FileRepository) Save(ctx context.Context, session *agent.Session) error {
    if err := ctx.Err(); err != nil {
        return fmt.Errorf("save session: %w", err)
    }
    if session == nil {
        return fmt.Errorf("save session: nil session")
    }

    path := filepath.Join(r.root, safeName(session.Key)+".jsonl")
    _ = path

    // TODO: create parent directory
    // TODO: write metadata line
    // TODO: write message lines
    return nil
}

func safeName(s string) string {
    // TODO: replace invalid path characters
    return s
}
```

### 7.4 `internal/service/conversation/service_test.go`

```go
package conversation

import (
    "context"
    "testing"
    "time"

    "tinybot/internal/domain/agent"
)

func TestServiceHandleInbound(t *testing.T) {
    t.Parallel()

    fixedNow := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)

    tests := []struct {
        name    string
        inbound agent.InboundMessage
        wantErr bool
    }{
        {
            name: "rejects empty content",
            inbound: agent.InboundMessage{
                Channel: "cli",
                ChatID:  "direct",
                Content: "   ",
            },
            wantErr: true,
        },
        {
            name: "accepts normal direct message",
            inbound: agent.InboundMessage{
                ID:      "msg-1",
                Channel: "cli",
                ChatID:  "direct",
                Content: "hello",
            },
            wantErr: false,
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            svc, err := New(
                stubSessionStore{},
                stubModelClient{},
                stubPromptBuilder{},
                stubToolCatalog{},
                fixedClock{now: fixedNow},
                4,
                4096,
            )
            if err != nil {
                t.Fatalf("New() error = %v", err)
            }

            _, err = svc.HandleInbound(context.Background(), tt.inbound)
            if (err != nil) != tt.wantErr {
                t.Fatalf("HandleInbound() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

The missing stubs are a good exercise:

- start with the smallest possible fake that satisfies each interface
- keep the fake deterministic
- add fields to capture calls only when a test needs them

## 8. Step-by-Step Implementation Guide

### Step 1 - Freeze compatible behavior

Before moving files, write down the behavior you must not break:

- direct CLI chat still works
- tool-call loop still repeats until a final answer
- sessions still persist across runs
- workspace bootstrap file names stay the same
- heartbeat still treats empty/comment-only files as no-op
- cron still updates `last_run`, `last_error`, and `next_run`

### Step 2 - Extract service-facing interfaces

Do this before any package move:

- shrink `internal/ports/` into smaller service-local interfaces
- remove methods that only concrete adapters need
- keep `context.Context` on all I/O or external calls

Good first extraction candidates:

- session store contract for conversation service
- model client contract
- tool catalog contract

### Step 3 - Separate prompt assembly from turn orchestration

Refactor `internal/usecase/chat/usecase.go` by moving prompt concerns into a
dedicated builder:

- `HandleInbound` should orchestrate
- prompt builder should assemble messages
- session repository should only load/save

This is the highest-value cleanup because it makes the agent core easier to
teach, test, and evolve.

### Step 4 - Rename packages toward the target layout

After behavior is covered by tests:

- move `internal/usecase/chat` -> `internal/service/conversation`
- move `internal/usecase/agent` -> `internal/service/runtime`
- move `internal/usecase/cron` and `internal/usecase/heartbeat` -> `internal/service/scheduler`
- move adapters into `internal/repository/*` and `internal/transport/*`

Keep the moves mechanical. Avoid logic changes during package relocation.

### Step 5 - Expand exported behavior tests

Prefer table-driven tests for exported entrypoints:

- `conversation.Service.HandleInbound`
- `prompt.Builder.Build`
- `session.FileRepository.GetOrCreate`
- `session.FileRepository.Save`
- `scheduler.Service.TriggerOnce`
- `scheduler.Heartbeat.TriggerOnce`
- `transport/cli.run`

### Step 6 - Add integration checks only after unit tests are stable

Once the unit boundaries are clean:

- direct CLI flow test
- session persistence round-trip test
- tool loop with fake model + fake tool registry

## 9. Concrete Next Task Recommendation

The best next coding task is:

1. refactor the current chat use case into a conversation service + prompt builder split
2. keep the existing JSONL session repository unchanged during that step
3. add table-driven tests around the new exported service

Why this should come next:

- it directly preserves the reference project's core behavior
- it improves the architecture without forcing a full rewrite
- it gives the cleanest teaching value for Go interfaces and dependency injection
- it reduces the risk of later cron/runtime/channel work

## 10. Test Plan

Run tests in this order after each meaningful refactor:

```bash
go test ./internal/usecase/chat
go test ./internal/usecase/agent
go test ./internal/usecase/cron
go test ./internal/usecase/heartbeat
go test ./...
```

If you move packages, keep the same test scenarios and update import paths
mechanically before changing logic.
