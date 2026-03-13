# Current State And Next Steps

This note is written against the current local refactor in this workspace.

It does three things:

1. summarize what the Go rewrite already implements
2. compare those pieces with the `nanobot/` reference project
3. give a beginner-friendly next-step plan without fighting the in-progress local changes

It is intentionally documentation-first. The current worktree already contains a
large package move from `ports/usecase/adapters` toward
`service/repository/transport`, so this note avoids forcing another structural
rewrite before the current direction is stabilized.

## 1. Current Local Refactor Context

The current worktree is already in the middle of an architecture cleanup:

- old `internal/ports/*` interfaces are being retired
- old `internal/usecase/*` packages are being replaced by `internal/service/*`
- old `internal/adapters/bus` and `internal/adapters/channel` have already moved under `internal/transport/*`
- old `internal/adapters/repository/*` has already been split into:
  - `internal/repository/sessionrepo`
  - `internal/repository/cronrepo`

That means the next useful step is not "start porting from zero".
The next useful step is "close the boundaries and preserve behavior".

## 2. What The Current Go Rewrite Already Has

### `cmd/tinybot`

Responsibilities:

- CLI entrypoint
- command dispatch
- direct chat path
- cron subcommands
- gateway startup

Current behavior:

- `tinybot <message>` runs a direct chat turn
- `tinybot gateway` starts the in-process runtime
- `tinybot cron list|add|remove|run-once` manages local scheduled jobs

### `internal/domain/model`

Responsibilities:

- define stable domain data
- keep business rules close to the data

Already implemented:

- `InboundMessage` / `OutboundMessage`
- `Session` / `Message`
- `LLMResponse` / `ToolCall`
- `CronJob` / `CronSchedule`

These models are already good enough to act as the stable center of the current
rewrite.

### `internal/service/chat`

Responsibilities:

- orchestrate one full agent turn
- load session history
- build prompt messages
- call the LLM
- execute tool calls in a loop
- save the turn trace

This is the closest Go equivalent to:

- `nanobot/nanobot/agent/loop.py`
- `nanobot/nanobot/agent/context.py`

### `internal/service/cron`

Responsibilities:

- load jobs
- detect due jobs
- execute each due job through the chat service
- persist updated cron state

This corresponds to the execution part of:

- `nanobot/nanobot/cron/service.py`

The periodic ticker loop has already been separated out into transport/runtime.

### `internal/service/heartbeat`

Responsibilities:

- read `HEARTBEAT.md`
- decide whether it contains actionable content
- call the chat service only when needed

This corresponds to:

- `nanobot/nanobot/heartbeat/service.py`

### `internal/repository/sessionrepo`

Responsibilities:

- JSONL session persistence
- session cache
- session listing helpers

This corresponds to:

- `nanobot/nanobot/session/manager.py`

### `internal/repository/cronrepo`

Responsibilities:

- `cron/jobs.json` load/save
- repository-edge validation of `model.CronJob`

This is the persistence half of:

- `nanobot/nanobot/cron/service.py`

### `internal/transport/*`

Responsibilities:

- `transport/bus`: in-process message bus
- `transport/gateway`: inbound -> chat service -> outbound loop
- `transport/channel`: channel runtime and outbound dispatch
- `transport/runtime`: periodic cron/heartbeat runners

This corresponds to the runtime and delivery parts of:

- `nanobot/nanobot/bus/*`
- `nanobot/nanobot/channels/*`
- the timer loops in `cron/service.py` and `heartbeat/service.py`

## 3. Reference `nanobot/` Module Analysis

This section follows the rewrite workflow:

1. responsibility
2. inputs / outputs
3. state transitions
4. side effects
5. Go mapping

### 3.1 `nanobot/agent/loop.py`

Responsibilities:

- receive inbound messages
- get or create a session
- assemble prompt messages
- call the model
- execute tool calls until the model stops requesting tools
- persist conversation state
- publish the final reply

Inputs:

- inbound message
- session history
- workspace context
- tool schemas
- tool outputs
- provider response

Outputs:

- outbound reply
- persisted session trace

State transitions:

1. wait for inbound message
2. resolve session
3. build prompt
4. call model
5. if tool calls exist: append assistant tool-call trace -> execute tools -> append tool results -> repeat
6. if no tool calls: finalize answer
7. save session
8. publish reply

Side effects:

- file I/O
- model network calls
- subprocess / filesystem / web tool effects
- outbound message publish

Go mapping:

- `internal/service/chat/service.go`
- `internal/transport/gateway/loop.go`

Compatibility points to preserve:

- one direct turn and one gateway turn should reuse the same chat orchestration
- tool execution failures should be returned to the model as text, not crash the process
- the tool loop should continue until the model stops asking for tools or the safety limit is hit

### 3.2 `nanobot/agent/context.py`

Responsibilities:

- assemble the system prompt
- read bootstrap files
- read memory context
- read skill metadata and active skill bodies
- merge history + current user message into a model-ready message list

Inputs:

- workspace path
- history
- current user message
- selected skill names

Outputs:

- model-facing `[]message`

State:

- references to workspace readers only

Side effects:

- file reads from workspace, memory, and skills directories

Go mapping:

- `internal/service/chat/prompt_builder.go`
- `internal/adapters/workspace/*`

Compatibility points to preserve:

- deterministic section order in the system prompt
- optional files should fail soft, not fail the whole turn
- progressive skill loading should remain "summary first, read full skill on demand"

### 3.3 `nanobot/session/manager.py`

Responsibilities:

- cache sessions
- load JSONL session files
- save JSONL session files
- expose session history

Inputs:

- session key
- session message list

Outputs:

- session object
- session metadata list

State:

- in-memory session cache

Side effects:

- file reads and writes

Go mapping:

- `internal/repository/sessionrepo/file_session_repo.go`

Compatibility points to preserve:

- stable session key -> stable file path
- load-or-create behavior
- JSONL metadata + message line layout

### 3.4 `nanobot/agent/tools/registry.py`

Responsibilities:

- register tools by name
- expose schemas to the model
- execute tools by name

Inputs:

- tool name
- tool parameters

Outputs:

- tool schema list
- tool result string

State:

- tool registry map

Side effects:

- delegated to tool implementations

Go mapping:

- `internal/adapters/tool/registry.go`
- `internal/adapters/tool/*.go`

Compatibility points to preserve:

- unknown tools and execution failures should come back as readable text
- registry ownership belongs to the chat-service boundary, not the transport layer

### 3.5 `nanobot/cron/service.py`

Responsibilities:

- load jobs from disk
- compute next run time
- schedule wakeups
- execute due jobs
- persist job state

Inputs:

- clock
- cron store path
- on-job callback

Outputs:

- updated job state
- optional delivery

State:

- in-memory loaded store
- timer lifecycle

Side effects:

- file I/O
- timer scheduling
- agent execution

Go mapping:

- execution and persistence: `internal/service/cron` + `internal/repository/cronrepo`
- runtime ticking: `internal/transport/runtime/cron.go`

Compatibility points to preserve:

- due jobs update `LastRunAt`, `LastResult`, `LastError`, and `NextRunAt`
- execution failures should still be persisted as job state

### 3.6 `nanobot/heartbeat/service.py`

Responsibilities:

- periodically inspect `HEARTBEAT.md`
- skip empty / comment-only / checkbox-only content
- invoke the agent if action is required

Inputs:

- workspace path
- heartbeat callback
- timer interval

Outputs:

- optional response text

State:

- runtime enabled flag
- timer lifecycle

Side effects:

- file reads
- timer scheduling
- agent execution

Go mapping:

- decision logic: `internal/service/heartbeat/service.go`
- periodic ticking: `internal/transport/runtime/heartbeat.go`

Compatibility points to preserve:

- empty or comment-only heartbeat file should be a no-op
- actionable heartbeat content should use a stable session key, currently `heartbeat`

## 4. Repository Closure: Who Uses What

This is the important "收口" map for the current refactor.

```text
service/chat      -> repository/sessionrepo
service/cron      -> repository/cronrepo
service/heartbeat -> no repository yet (reads HEARTBEAT.md directly)
transport/*       -> no repository; runtime/message delivery only
cmd/tinybot       -> can depend on repository/cronrepo only for CRUD-like CLI flows
```

### `internal/repository/sessionrepo`

Used by:

- `internal/service/chat`
- `internal/app/bootstrap.go`

Owns:

- session file path rules
- JSONL metadata/message encoding
- session cache
- session list helpers

Should not own:

- prompt building
- tool execution
- model calls
- cron logic

### `internal/repository/cronrepo`

Used by:

- `internal/service/cron`
- `cmd/tinybot/run.go`
- `internal/app/gateway.go`

Owns:

- `cron/jobs.json` storage
- full job list load/save
- repository-edge validation

Should not own:

- polling interval
- runtime ticker
- agent execution
- CLI output formatting

## 5. Recommended Package Layout From Here

The current code already mostly matches the desired structure.
Do not restart the layout.
Tighten the remaining boundaries instead.

```text
cmd/
  tinybot/

internal/
  domain/
    errors/
    model/

  service/
    chat/
    cron/
    heartbeat/

  repository/
    sessionrepo/
    cronrepo/

  transport/
    bus/
    channel/
    gateway/
    runtime/

  adapters/
    provider/
    tool/
    workspace/

  app/
```

### Why keep `adapters/provider`, `adapters/tool`, and `adapters/workspace` for now

- they are already wired and tested
- moving them again right now adds churn without new learning value
- the urgent cleanup is the service/repository boundary, not package renaming

If the current refactor remains stable, a later cleanup can move:

- `adapters/provider` -> `repository/provider` or `repository/llm`
- `adapters/workspace` -> `repository/workspace`

But that should happen only after the chat service contract is frozen.

## 6. Interface Design And Skeletons

These skeletons are meant for guided implementation.
They are intentionally close to the current code instead of inventing a brand new architecture.

### 6.1 `internal/service/chat/contracts.go`

```go
package chat

import (
    "context"

    "tinybot/internal/domain/model"
)

// SessionRepository is the smallest persistence boundary that chat.Service needs.
// Next cleanup target:
// - add context.Context to GetOrCreateSession and SaveSession
// - keep file format details out of the service layer
type SessionRepository interface {
    GetOrCreateSession(key string) *model.Session
    SaveSession(session *model.Session) error
}

// CompletionClient hides Qwen or any future provider details from the service.
type CompletionClient interface {
    Chat(
        ctx context.Context,
        messages []map[string]any,
        tools []map[string]any,
        maxTokens int,
        temperature float32,
    ) (model.LLMResponse, error)
}

// ToolExecutor owns tool schema exposure and tool execution.
type ToolExecutor interface {
    GetDefinitions() []map[string]any
    Execute(ctx context.Context, name string, params map[string]any) (string, error)
}

// PromptBuilder owns model-facing message assembly.
type PromptBuilder interface {
    BuildMessages(history []*model.Message, currentMessage string, skillNames []string) []map[string]any
    AddAssistantMessage(messages []map[string]any, content string, toolCalls []map[string]any) []map[string]any
    AddToolResult(messages []map[string]any, toolCallID string, toolName string, result string) []map[string]any
}
```

### 6.2 `internal/service/chat/service.go`

```go
package chat

import (
    "context"
    "fmt"
    "strings"
    "time"

    "tinybot/internal/domain/model"
)

// Service orchestrates one complete chat turn.
// Keep it focused on orchestration only:
// - load session
// - build prompt
// - call model
// - execute tools
// - persist trace
type Service struct {
    sessions      SessionRepository
    llm           CompletionClient
    tools         ToolExecutor
    prompts       PromptBuilder
    maxIterations int
    maxTokens     int
    temperature   float32
}

func (s *Service) ProcessMessage(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error) {
    if strings.TrimSpace(msg.Content) == "" {
        return model.OutboundMessage{}, fmt.Errorf("chat service: message content is empty")
    }

    session := s.sessions.GetOrCreateSession(msg.SessionKey())
    if session == nil {
        return model.OutboundMessage{}, fmt.Errorf("chat service: failed to get or create session")
    }

    llmMessages := s.prompts.BuildMessages(session.GetHistory(500), msg.Content, nil)

    // TODO:
    // 1. call llm
    // 2. if response has tool calls:
    //    - append assistant tool-call trace
    //    - execute each tool
    //    - append tool result trace
    //    - continue
    // 3. otherwise finalize assistant answer
    // 4. save user/tool/assistant trace into session

    return model.OutboundMessage{
        Channel:   msg.Channel,
        ChatID:    msg.ChatID,
        ReplyTo:   msg.ID,
        Content:   "",
        Metadata:  map[string]any{"session_key": session.Key},
        CreatedAt: time.Now(),
    }, nil
}

func (s *Service) ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error) {
    // Keep this as a thin wrapper over ProcessMessage so CLI / cron / heartbeat
    // all reuse the same orchestration path.
    _ = ctx
    _ = sessionKey
    _ = content
    return "", nil
}
```

### 6.3 `internal/service/chat/prompt_builder.go`

```go
package chat

import "tinybot/internal/domain/model"

// Builder should stay dumb and deterministic.
// It should read context and turn it into prompt messages, not call the model.
type Builder struct {
    workspacePath string
}

func (b *Builder) BuildMessages(history []*model.Message, currentMessage string, skillNames []string) []map[string]any {
    // TODO:
    // 1. add system prompt
    // 2. append history
    // 3. append current user message
    return nil
}

func (b *Builder) BuildSystemPrompt(skillNames []string) string {
    // TODO:
    // identity -> bootstrap docs -> memory -> always skills -> skills summary
    _ = skillNames
    return ""
}
```

### 6.4 `internal/repository/sessionrepo/file_session_repo.go`

```go
package sessionrepo

import (
    "fmt"
    "path/filepath"

    "tinybot/internal/domain/model"
)

// FileSessionRepository owns JSONL session persistence only.
type FileSessionRepository struct {
    WorkSpace  string
    SessionDir string
    cache      map[string]*model.Session
}

func NewFileSessionRepository(workspace string) *FileSessionRepository {
    return &FileSessionRepository{
        WorkSpace:  workspace,
        SessionDir: filepath.Join(workspace, "sessions"),
        cache:      make(map[string]*model.Session),
    }
}

func (r *FileSessionRepository) GetOrCreateSession(key string) *model.Session {
    // TODO:
    // 1. check cache
    // 2. load JSONL from disk
    // 3. fall back to model.NewSession(key)
    return nil
}

func (r *FileSessionRepository) SaveSession(session *model.Session) error {
    if session == nil {
        return fmt.Errorf("session repository: nil session")
    }

    // TODO:
    // 1. ensure session dir exists
    // 2. write metadata line
    // 3. write message lines
    // 4. refresh cache
    return nil
}
```

### 6.5 `internal/service/cron/contracts.go`

```go
package cron

import (
    "context"

    "tinybot/internal/domain/model"
)

// Repository is only for durable job state.
type Repository interface {
    ListJobs(ctx context.Context) ([]model.CronJob, error)
    SaveJobs(jobs []model.CronJob) error
}

// AgentTurner lets cron reuse the chat service.
type AgentTurner interface {
    ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error)
}
```

### 6.6 `internal/service/heartbeat/service.go`

```go
package heartbeat

import "context"

// Service should remain a single-pass decision layer:
// - read HEARTBEAT.md
// - decide actionable or not
// - call AgentTurner when needed
//
// The ticker belongs in transport/runtime, not here.
type Service struct {
    workspace string
    agent     AgentTurner
}

func (s *Service) TriggerOnce(ctx context.Context) (string, error) {
    _ = ctx
    return "", nil
}
```

## 7. Step-By-Step Implementation Guide

This order is chosen for learning value and low refactor risk.

### Step 1: freeze the current chat boundary

What to do:

- keep `service/chat` as the only place that orchestrates a turn
- keep `repository/sessionrepo` as the only session persistence implementation
- do not move provider/tool/workspace packages yet

Why next:

- this is the center of the agent
- cron, heartbeat, and gateway all depend on this path

Where:

- `internal/service/chat/*`
- `internal/repository/sessionrepo/*`

### Step 2: add `context.Context` to chat-facing repository I/O

What to do:

- change `GetOrCreateSession(key string)` to `GetOrCreateSession(ctx context.Context, key string)`
- change `SaveSession(session *model.Session)` to `SaveSession(ctx context.Context, session *model.Session)`

Why next:

- it matches the project rule "always propagate context"
- it makes future tracing and cancellation easier

Where:

- `internal/service/chat/contracts.go`
- `internal/repository/sessionrepo/file_session_repo.go`
- `internal/app/bootstrap.go`
- tests for chat service and session repo

### Step 3: stabilize prompt builder behavior with table-driven tests

What to do:

- keep the current order:
  - identity
  - bootstrap files
  - memory
  - always skills
  - skills summary
- write tests for missing-file soft failure and stable ordering

Why next:

- it is deterministic
- it is beginner-friendly
- it gives fast feedback without touching provider code

Where:

- `internal/service/chat/prompt_builder.go`
- `internal/service/chat/prompt_builder_test.go`

### Step 4: close repository ownership for chat and cron

What to do:

- keep session helpers in `sessionrepo`
- keep cron store helpers in `cronrepo`
- do not let chat service know JSONL details
- do not let cron service know file path details

Why next:

- this is the exact "repository 收口" the current refactor is aiming for

Where:

- `internal/repository/sessionrepo/*`
- `internal/repository/cronrepo/*`
- `docs/porting-notes/repository-boundaries.md`

### Step 5: improve cron compatibility incrementally

What to do:

- keep current `every` behavior working first
- later add `at`
- later add `cron` expression support
- only after that think about `deliver/channel/to`

Why next:

- current Go cron is intentionally smaller than Python nanobot
- schedule expansion is easier after the repository boundary is stable

Where:

- `internal/domain/model/cron.go`
- `internal/service/cron/service.go`
- `internal/repository/cronrepo/file_cron_repo.go`

### Step 6 update: split and wire the message tool by runtime mode

Current state:

- `internal/app/bootstrap.go` keeps a gateway-independent core tool set in `buildCoreToolRegistry(...)`
- `internal/app/gateway.go` registers `message` only for gateway mode and wires it to `PublishOutbound(...)`
- `internal/service/chat/service.go` updates message-tool channel/chat context per inbound message before the tool loop runs
- tests now cover both halves of the split:
  - base/direct tool registry does not expose `message`
  - gateway runtime wiring publishes `message` tool output through the outbound bus

Why this split exists:

- direct CLI uses `ProcessDirect(...)` and writes the final assistant reply straight to stdout
- direct CLI does not own an outbound transport bus or a channel manager, so exposing `message` there would advertise a tool that cannot actually deliver anything
- gateway mode owns the runtime pieces that can deliver outbound messages: message bus, gateway loop, and channel manager
- keeping `message` runtime-scoped preserves a clean boundary: shared app/core code owns safe common tools, gateway owns transport-dependent tools

Where:

- `internal/app/bootstrap.go`
  - `buildCoreToolRegistry(...)` defines the base tool set used by direct chat and shared app setup
- `internal/app/gateway.go`
  - `NewGatewayApp(...)` registers `message` and connects it to the outbound bus
- `internal/service/chat/service.go`
  - `ProcessMessage(...)` injects the current inbound channel/chat into the tool runtime before executing tool calls
- `internal/adapters/tool/message_tool.go`
  - `Execute(...)` uses explicit `channel/chat_id` when provided, otherwise falls back to the current runtime context

Rule for future channels:

- when adding Telegram, WhatsApp, or other long-running transports, keep `message` registration in the runtime assembly that owns outbound delivery
- do not move `message` back into the shared core registry unless the base app also grows a real outbound transport contract

## 8. Current Behavior Differences Versus `nanobot`

These are the main gaps or intentional differences visible today.

### Intentional differences already present in Go

- `ProcessDirect` preserves an explicit session key in Go.
  - This is useful for CLI, cron, and heartbeat integration.
- Go persists assistant tool-call traces and tool-result traces in sessions.
  - Python currently saves a smaller conversation trace.
- `message` is runtime-scoped in Go.
  - base/direct app setup exposes only transport-safe core tools
  - gateway mode adds `message` and routes it through the outbound bus
  - this avoids exposing a send-only tool in modes that have no outbound delivery path

### Missing compatibility pieces

- cron only supports `every`
  - Python also supports `at` and cron expressions
- cron does not yet model delivery payloads like `deliver/channel/to`
- chat-facing session repository methods still do not accept `context.Context`

### Lower-risk differences to accept for now

- provider support is narrower in Go
  - current focus is Qwen-compatible behavior
- transport support is narrower in Go
  - console channel exists, Telegram/WhatsApp are not yet ported

## 9. Test Plan

The current codebase already has useful exported-behavior tests.
Keep extending them instead of rewriting them.

Recommended test focus:

- `internal/service/chat`
  - empty input
  - prompt builder usage
  - tool loop regression
  - tool failure round-trip
  - configured token / temperature propagation
- `internal/service/cron`
  - due job execution
  - error recording
- `internal/service/heartbeat`
  - missing file
  - comment-only file
  - actionable file
- `internal/repository/sessionrepo`
  - save/load round-trip
  - cache + invalidate behavior
- `internal/repository/cronrepo`
  - save/list round-trip
  - invalid JSON handling
- `cmd/tinybot`
  - direct chat path
  - cron commands

Recommended commands after each meaningful change:

```powershell
go test ./internal/service/...
go test ./internal/repository/...
go test ./internal/transport/...
go test ./cmd/tinybot
go test ./...
```

## 10. Best Next Coding Task For A Beginner

If only one next task should be implemented now, choose this:

1. add `context.Context` to `service/chat` <-> `repository/sessionrepo`
2. keep the JSONL format unchanged
3. update tests in table-driven style

Why this is the best next task:

- it improves the architecture without changing user-visible behavior
- it teaches interface design, dependency injection, and context propagation
- it is small enough to finish in one milestone
- it reduces risk before the next cron and transport changes
