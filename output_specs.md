# Software Specification — tinybot

## 1. Executive Overview

**tinybot** is a Go rewrite of the Python [nanobot](https://github.com/HKUDS/nanobot) project — an ultra-lightweight personal AI assistant that combines LLM-powered chat with tool execution, scheduled tasks, persistent memory, and multi-channel messaging.

The system receives natural-language messages from a user (via CLI or a gateway console channel), assembles a context-aware prompt that includes workspace documents, memory, and skill metadata, sends the prompt to an LLM provider, executes any tool calls the model requests, and returns the final reply. Conversations are persisted as JSONL files. A background gateway mode adds scheduled cron jobs and periodic heartbeat evaluation.

### Who uses it

A single developer-operator who runs tinybot locally. The current implementation targets a CLI user on Windows or Linux. The operator is described in `AGENTS.md` as a Go beginner using the rewrite to learn agent architecture.

### What problem it solves

It provides a personal AI assistant that can read/write/edit files, execute shell commands, search the web, fetch web pages, remember long-term context, and run scheduled background tasks — all from a minimal, self-contained Go binary with no external database.

### What outcomes it supports

- Interactive direct-message chat with an LLM that has tool-use capability
- Long-running gateway mode with a console channel, cron runner, and heartbeat monitor
- Persistent conversation history via JSONL sessions
- Workspace-based configuration, memory, and skills
- Workspace onboarding and status inspection

### What is out of scope or not evidenced

- **Telegram / WhatsApp channel adapters**: defined in the domain model (`ChannelTelegram`, `ChannelWhatsApp`) but no concrete Go adapter code exists. The Python nanobot reference includes these.
- **Multi-provider LLM support**: the code only implements a Qwen/DashScope-compatible provider. Config structs have TODO comments for multiple providers.
- **Multi-modal input**: media URL fields exist on `InboundMessage` but no processing code handles them.
- **HTTP/REST API gateway**: no HTTP server or REST endpoint exists.
- **Authentication or multi-tenancy**: the system is single-user with no auth layer.
- **Progressive skill activation**: skill names are accepted in method signatures but the current implementation only uses always-on skills and summaries.

---

## 2. Repository Context and Scope

### Repository purpose

This is a learning-oriented Go rewrite of `nanobot/`, a Python AI assistant. The `nanobot/` directory is embedded as a read-only reference. The Go code is the active project.

### System type

**CLI application** with an optional long-running local gateway mode. It is a single-binary monolith, not a microservice.

### Major directories and their roles

| Directory | Role |
|-----------|------|
| `cmd/tinybot/` | CLI entrypoint, command dispatch, and integration with the app layer |
| `internal/app/` | Composition root: bootstrap, config, onboard, status, gateway wiring |
| `internal/domain/model/` | Core domain types: Session, Message, LLMResponse, ToolCall, CronJob |
| `internal/domain/errors/` | Shared domain-level sentinel errors |
| `internal/service/chat/` | Chat turn orchestration, prompt building, service-layer contracts |
| `internal/service/cron/` | Cron trigger-once evaluation and job state management |
| `internal/service/heartbeat/` | Heartbeat file evaluation and conditional agent invocation |
| `internal/repository/sessionrepo/` | JSONL file-backed session persistence |
| `internal/repository/cronrepo/` | JSON file-backed cron job persistence |
| `internal/transport/` | Transport contracts (MessageBus, Channel, MessageProcessor) |
| `internal/transport/bus/` | In-memory channel-based message bus |
| `internal/transport/channel/` | Console channel and channel manager |
| `internal/transport/gateway/` | Gateway loop bridging bus to chat service |
| `internal/transport/runtime/` | Periodic ticker runners for cron and heartbeat |
| `internal/adapters/provider/` | Qwen LLM provider adapter (OpenAI-compatible API) |
| `internal/adapters/tool/` | Built-in tool implementations and registry |
| `internal/adapters/workspace/` | Bootstrap reader, memory store, skills loader |
| `internal/utils/` | Small utility functions (path expansion, date) |
| `internal/ports/` | Legacy interface package being retired (files deleted in working tree) |
| `internal/usecase/` | Legacy use-case package being retired (files deleted in working tree) |
| `nanobot/` | Python reference project (read-only) |
| `docs/porting-notes/` | Porting decision records |
| `.tinybot/workspace/` | Active workspace with config, sessions, memory, skills |

### Scope of this spec

This spec covers the entire Go codebase (`cmd/` and `internal/`). The Python `nanobot/` reference is used for context but is not the subject of the specification.

---

## 3. System Boundary Definition

### What is inside the system

- CLI command dispatch and argument parsing
- LLM chat orchestration including iterative tool-call loops
- Prompt assembly from workspace files, memory, and skills
- Session persistence (JSONL)
- Cron job persistence (JSON) and one-pass trigger evaluation
- Heartbeat file evaluation
- In-memory message bus for gateway mode
- Console channel (stdin/stdout transport)
- Tool execution: file I/O, shell exec, web search, web fetch, message send
- Workspace onboarding and status checking
- Configuration loading (JSON file + environment variable overrides)

### What is outside the system

- The LLM model itself — tinybot calls an external OpenAI-compatible API (Qwen/DashScope)
- Web search API (Brave Search or similar, accessed via HTTP)
- The operating system shell (invoked by the exec tool)
- External chat platforms (Telegram, WhatsApp) — not yet implemented in Go
- Any persistent database — storage is entirely file-based

### External actors

| Actor | Description |
|-------|-------------|
| **User** | Sends messages via CLI or console channel |
| **LLM Provider** | Qwen/DashScope API returning completions with optional tool calls |
| **Web Search API** | Brave Search or compatible, returning search results as JSON |
| **File System** | Workspace directory tree for config, sessions, memory, skills, heartbeat, cron |
| **OS Shell** | Command execution via the exec tool |

### Data entering the system

- User text messages (CLI args or stdin lines)
- LLM API responses (text + tool calls)
- Web search API responses
- Web page HTML content
- Workspace file contents (AGENTS.md, SOUL.md, MEMORY.md, SKILL.md, HEARTBEAT.md, config.json, session JSONL, cron jobs.json)
- Environment variables (QWEN_API_KEY, QWEN_API_BASE, QWEN_MODEL, WEB_SEARCH_API_KEY, etc.)

### Data leaving the system

- Assistant reply text (stdout or console channel)
- LLM API requests
- Web search API requests
- Web fetch HTTP requests
- File system writes (sessions, cron state, user-created files via tools)
- Shell command executions

---

## 4. Core Capabilities and Feature Areas

### 4.1 Direct Chat

The primary user-facing capability. The user passes a message as CLI arguments, and the system returns the assistant's reply.

**Flow**: `CLI args → app bootstrap → session load → prompt build → LLM call → (tool loop) → session save → print reply`

**Constraints**: 90-second context timeout. Session key is `cli:direct`. Max 20 tool iterations per turn (configurable).

**Dependencies**: LLM provider, session repository, tool registry, prompt builder.

### 4.2 Gateway Mode

A long-running local runtime that reads from stdin, processes messages through the bus, and writes replies to stdout. Also runs cron and heartbeat background loops.

**Flow**: `stdin → console channel → bus → gateway loop → chat service → bus → console channel → stdout`

**Constraints**: Runs until Ctrl+C (SIGINT). Four concurrent goroutines: gateway loop, channel manager, heartbeat runner, cron runner.

**Dependencies**: All of direct chat plus message bus, channel manager, cron service, heartbeat service.

### 4.3 Tool System

Eight built-in tools that the LLM can invoke via function calling:

| Tool | Description | Key constraint |
|------|-------------|----------------|
| `exec` | Execute shell commands | Configurable timeout (default 10s), workspace-scoped |
| `read_file` | Read file contents | Workspace-relative path resolution |
| `write_file` | Write/create files | Workspace-relative path resolution |
| `edit_file` | String replacement in files | Requires old_string to be unique |
| `list_dir` | List directory contents | Workspace-relative path resolution |
| `web_search` | Search the web via Brave Search API | Max results configurable (default 10) |
| `web_fetch` | Fetch and extract readable content from a URL | Max chars configurable (default 50000) |
| `message` | Send a message to a chat channel | Channel-aware, used for cross-channel messaging |

**Observed**: All tools implement a `Tool` interface with `Spec()` and `Execute(ctx, params)`. The registry provides `GetDefinitions()` for LLM schema exposure and `Execute()` for dispatch.

### 4.4 Prompt Assembly

The prompt builder composes the system prompt from multiple sources in a fixed order:

1. **Identity template** — hardcoded tinybot identity with current time and workspace path
2. **Bootstrap docs** — AGENTS.md, SOUL.md, USER.md, TOOLS.md, IDENTITY.md (optional)
3. **Memory** — long-term memory (MEMORY.md) and today's daily notes
4. **Always-on skills** — full body content of skills marked `always: true`
5. **Skills summary** — XML-formatted summary of all available skills for progressive loading

Missing optional files degrade gracefully without aborting the chat turn.

### 4.5 Session Persistence

Sessions are stored as JSONL files. Each file has a metadata line followed by message lines. Sessions are identified by a key in the format `{channel}:{chatID}` (e.g., `cli:direct`, `telegram:12345`).

**Features**: in-memory cache, load-or-create semantics, history windowing with `GetHistory(n)` that trims orphan leading non-user messages.

### 4.6 Cron Scheduling

Cron jobs are persisted in `cron/jobs.json`. The cron service evaluates due jobs by calling `ProcessDirect` on the chat service. The `CronRunner` in the transport runtime layer owns the periodic ticker.

**CLI commands**: `cron list`, `cron add <name> <every_seconds> <prompt>`, `cron remove <job_id>`, `cron run-once`.

**Current limitation**: Only `every` schedule kind is implemented. Python nanobot also supports `at` and `cron` expressions.

### 4.7 Heartbeat

The heartbeat service reads `HEARTBEAT.md` from the workspace. If it contains actionable content (not just headers, comments, or empty checkboxes), it triggers a direct chat turn with a fixed prompt asking the agent to follow the instructions. Otherwise it returns `HEARTBEAT_OK`.

### 4.8 Workspace Onboarding

The `onboard` command creates the workspace directory structure and default files:
- `AGENTS.md`, `SOUL.md`, `USER.md`, `TOOLS.md` (bootstrap documents)
- `memory/MEMORY.md` (long-term memory)
- `HEARTBEAT.md` (heartbeat instructions)
- `config.json` (default configuration)
- `skills/` directory
- Does not overwrite existing files.

### 4.9 Status Inspection

The `status` command reports whether key workspace files and directories exist, and lists any missing required files.

---

## 5. Use Cases and User Roles

### Actor: User (single developer-operator)

#### UC1: Direct Chat

- **Preconditions**: Workspace initialized, LLM API key configured
- **Main flow**: User runs `tinybot <message>`. System builds prompt, calls LLM, handles any tool calls, saves session, prints reply.
- **Alternative flows**: LLM returns empty content → fallback message "Sorry, I encountered an error calling the AI model."
- **Failure cases**: LLM API error → error returned to user. Session save error → error returned to user. 90-second timeout → context deadline exceeded.
- **Postconditions**: Session trace persisted to JSONL.

#### UC2: Gateway Mode

- **Preconditions**: Workspace initialized, LLM API key configured
- **Main flow**: User runs `tinybot gateway`. System starts bus, console channel, gateway loop, heartbeat runner, cron runner. User types messages at the prompt.
- **Alternative flows**: LLM error → fallback error message sent to console. Heartbeat/cron errors → swallowed, gateway continues.
- **Failure cases**: Bus close or context cancellation → graceful shutdown.
- **Postconditions**: Sessions persisted. Cron job state updated.

#### UC3: Manage Cron Jobs

- **Preconditions**: Workspace directory accessible
- **Main flow**: User runs `tinybot cron add daily 60 "check status"`. System validates, persists job to `cron/jobs.json`.
- **Alternative flows**: `cron list` shows all jobs. `cron remove <id>` deletes a job. `cron run-once` triggers one evaluation pass.
- **Failure cases**: Invalid args → error. Job not found for remove → error.
- **Postconditions**: Job list updated on disk.

#### UC4: Initialize Workspace

- **Preconditions**: None
- **Main flow**: User runs `tinybot onboard`. System creates workspace, default files, and config.
- **Alternative flows**: Re-running onboard does not overwrite existing files.
- **Postconditions**: Workspace ready with all required files.

#### UC5: Check Status

- **Preconditions**: None
- **Main flow**: User runs `tinybot status`. System reports workspace existence, config presence, memory file presence, skills directory presence, heartbeat file presence, and missing files.
- **Postconditions**: None (read-only).

---

## 6. Domain Model and Behavioral Rules

### Entities

#### Session

- **Fields**: Key (string, format `{channel}:{chatID}`), Messages ([]*Message), CreatedAt, UpdatedAt, Metadata (map), LastConsolidated (int)
- **Invariant**: Messages are append-only during a turn. UpdatedAt is refreshed on every AddMessage.
- **Lifecycle**: Created on first access via GetOrCreateSession. Persisted after each completed turn. Loaded from JSONL on subsequent access.
- **History trimming**: `GetHistory(n)` returns at most `n` unconsolidated messages, trimming leading non-user messages to avoid orphan tool chains.

#### Message

- **Fields**: Role (user/assistant/tool/system), Content, CreatedAt, ToolCalls (any), ToolCallID, Name
- **Roles follow OpenAI convention**: user messages are input, assistant messages are LLM output, tool messages are tool results, system messages set context.

#### InboundMessage

- **Fields**: ID, Channel, SenderID, ChatID, Content, MediaURLs, Metadata, CreatedAt, SessionKeyOverride
- **SessionKey derivation**: `SessionKeyOverride` takes precedence; otherwise `"{channel}:{chatID}"`.

#### OutboundMessage

- **Fields**: ID, Channel, ChatID, Content, ReplyTo, Metadata, CreatedAt

#### CronJob

- **Fields**: ID, Name, Enabled, Schedule (Kind + EverySeconds), Prompt, DeliverTo, SessionKey, CreatedAt, UpdatedAt, NextRunAt, LastRunAt, LastError, LastResult
- **Invariant**: ID, Name, and Prompt must be non-empty. Schedule must be `every` kind with positive seconds.
- **Lifecycle states**: Created → Enabled (IsDue checks NextRunAt vs now) → Executed (LastRunAt/NextRunAt updated) → potentially Disabled.
- **Validation**: `Validate()` enforces structural rules at both domain and repository boundaries.

#### LLMResponse

- **Fields**: Content, ToolCalls, FinishReason, PromptTokens, OutputTokens
- **HasToolCalls()**: returns true when ToolCalls is non-empty, triggering the tool loop.

#### ToolCall

- **Fields**: ID, Name, Args (map)
- The ID is used to correlate tool results back to the LLM conversation.

### Channel enum

Five channel types are defined: `cli`, `telegram`, `whatsapp`, `cron`, `heartbeat`. Only `cli` has a concrete transport implementation.

### Behavioral Rules

1. **Tool loop iteration limit**: The chat service runs at most `maxIterations` (default 20) tool-call rounds per turn. If the model keeps requesting tools beyond this limit, the turn ends with the fallback message.
2. **Soft failure for optional context**: Missing bootstrap files, memory, or skills do not abort prompt building.
3. **Hard failure for core dependencies**: Missing config, LLM provider, or session repository cause startup failure.
4. **Workspace skill priority**: Workspace skills override builtin skills of the same name.
5. **Session key override**: Direct CLI calls use an explicit session key (`cli:direct`) rather than deriving from channel/chatID.
6. **Heartbeat gating**: The heartbeat service only invokes the LLM if `HEARTBEAT.md` contains actionable content (not just headers, comments, or empty checkboxes).

---

## 7. Component and Module Overview

### `cmd/tinybot`

- **Responsibility**: CLI entrypoint and command dispatch
- **Inputs**: `os.Args`, stdin/stdout
- **Outputs**: Formatted text to stdout, error to stderr
- **Dependencies**: `internal/app` (bootstrap, gateway, onboard, status), `internal/service/cron`, `internal/repository/cronrepo`
- **Owns**: Command routing, CLI output formatting, direct chat timeout (90s)
- **Does not own**: Business logic, persistence, LLM calls

### `internal/app`

- **Responsibility**: Composition root — wires all dependencies together
- **Key types**: `App` (direct chat), `GatewayApp` (long-running), `Config`, `OnBoardResult`, `Status`
- **Owns**: Dependency injection, config loading/saving, workspace path resolution, onboard file creation, status checking
- **Does not own**: Chat orchestration, tool execution, transport concerns

### `internal/service/chat`

- **Responsibility**: Chat turn orchestration — the core agent loop
- **Key types**: `Service`, `Builder` (prompt builder)
- **Contracts owned**: `SessionRepository`, `CompletionClient`, `ToolExecutor`, `PromptBuilder`, `BootstrapReader`, `MemoryReader`, `SkillCatalog`
- **Inputs**: InboundMessage or (sessionKey, content) pair
- **Outputs**: OutboundMessage or reply string
- **Owns**: Turn lifecycle (session load → prompt → LLM → tool loop → save), prompt assembly order
- **Does not own**: Persistence format, LLM API details, tool implementations

### `internal/service/cron`

- **Responsibility**: One-pass cron evaluation
- **Contracts owned**: `Repository`, `AgentTurner`
- **Owns**: Due-job detection, job state mutation (LastRunAt, NextRunAt, LastError, LastResult)
- **Does not own**: Periodic scheduling (that's `transport/runtime`), persistence format

### `internal/service/heartbeat`

- **Responsibility**: Evaluate HEARTBEAT.md and optionally trigger a chat turn
- **Contracts owned**: `AgentTurner`
- **Owns**: File content analysis, actionability decision
- **Does not own**: Periodic scheduling, file format

### `internal/repository/sessionrepo`

- **Responsibility**: JSONL session persistence with in-memory cache
- **Implements**: `chat.SessionRepository`
- **Owns**: JSONL format, file path derivation, cache lifecycle

### `internal/repository/cronrepo`

- **Responsibility**: JSON cron job persistence
- **Implements**: `cron.Repository`
- **Owns**: `cron/jobs.json` format, validation at repository boundary

### `internal/transport`

- **Responsibility**: Transport-layer contracts and implementations
- **Key contracts**: `MessageBus`, `MessageProcessor`, `Channel`
- **Subpackages**: `bus/` (MemoryBus), `channel/` (ConsoleChannel, ChannelManager), `gateway/` (Loop), `runtime/` (CronRunner, HeartbeatRunner)
- **Owns**: In-process message routing, stdin/stdout I/O, periodic ticker scheduling
- **Does not own**: Business logic, persistence

### `internal/adapters/provider`

- **Responsibility**: LLM provider adapter
- **Implements**: `chat.CompletionClient`
- **Owns**: OpenAI-compatible API call translation (using `github.com/openai/openai-go`)

### `internal/adapters/tool`

- **Responsibility**: Built-in tool implementations and registry
- **Implements**: `chat.ToolExecutor` (via Registry)
- **Key contracts owned**: `Tool` interface (Spec + Execute), `ToolSpec`, `ToolParam`
- **Owns**: Tool schema definitions, execution logic, workspace-relative path resolution

### `internal/adapters/workspace`

- **Responsibility**: Workspace file readers for prompt building
- **Implements**: `chat.BootstrapReader`, `chat.MemoryReader`, `chat.SkillCatalog`
- **Owns**: Bootstrap file loading, memory context formatting, skill discovery/parsing/summary

---

## 8. API and Interface Contracts

### CLI Commands

| Command | Method | Description |
|---------|--------|-------------|
| `tinybot <message...>` | Direct chat | Joins args as message text, sends to LLM, prints reply |
| `tinybot help` / `-h` / `--help` | Help | Prints usage |
| `tinybot onboard` | Onboard | Creates workspace with default files |
| `tinybot status` | Status | Reports workspace file existence |
| `tinybot gateway` | Gateway | Starts long-running console gateway |
| `tinybot cron list` | Cron list | Lists all cron jobs |
| `tinybot cron add <name> <every_seconds> <prompt...>` | Cron add | Creates a new cron job |
| `tinybot cron remove <job_id>` | Cron remove | Deletes a cron job by ID |
| `tinybot cron run-once` | Cron trigger | Evaluates and executes due cron jobs once |

### Service-Layer Interfaces (Observed)

```go
// chat.SessionRepository
GetOrCreateSession(ctx context.Context, key string) (*model.Session, error)
SaveSession(ctx context.Context, session *model.Session) error

// chat.CompletionClient
Chat(ctx context.Context, messages []map[string]any, tools []map[string]any, maxTokens int, temperature float32) (model.LLMResponse, error)

// chat.ToolExecutor
GetDefinitions() []map[string]any
Execute(ctx context.Context, name string, params map[string]any) (string, error)

// chat.PromptBuilder
BuildMessages(history []*model.Message, currentMessage string, skillNames []string) []map[string]any
AddAssistantMessage(messages []map[string]any, content string, toolCalls []map[string]any) []map[string]any
AddToolResult(messages []map[string]any, toolCallID string, toolName string, result string) []map[string]any

// transport.MessageBus
PublishInbound(ctx context.Context, msg model.InboundMessage) error
ConsumeInbound(ctx context.Context) (model.InboundMessage, error)
PublishOutbound(ctx context.Context, msg model.OutboundMessage) error
ConsumeOutbound(ctx context.Context) (model.OutboundMessage, error)
Close() error

// transport.MessageProcessor
ProcessMessage(ctx context.Context, msg model.InboundMessage) (model.OutboundMessage, error)

// transport.Channel
Name() model.Channel
Start(ctx context.Context) error
Send(ctx context.Context, out model.OutboundMessage) error
```

### Tool Schemas (Partial)

Each tool exposes an OpenAI function-calling-compatible JSON schema via `Spec()`. Parameters vary by tool. The schemas are defined inline in each tool's Go file using `ToolSpec` and `ToolParam` structs.

---

## 9. Data and Persistence Design

### Storage technologies

**File-system only.** No database, no external key-value store.

### Session storage

- **Format**: JSONL (one JSON object per line)
- **Location**: `{workspace}/sessions/{safe_key}.jsonl`
- **Structure**:
  - Line 1: Metadata object (`_type: "metadata"`, key, metadata map, timestamps, last_consolidated)
  - Line 2+: Message objects (role, content, created_at, tool_calls, tool_call_id, name)
- **Key derivation**: Channel colons replaced with underscores, unsafe path characters replaced
- **Caching**: In-memory map keyed by session key, invalidated manually

### Cron job storage

- **Format**: JSON array
- **Location**: `{workspace}/cron/jobs.json`
- **Structure**: Array of `CronJob` objects with schedule, timestamps, and execution state

### Configuration storage

- **Format**: JSON object
- **Location**: `{workspace}/config.json`
- **Structure**: Nested config with `agents`, `channels`, `providers`, `tools`, `heartbeat` sections
- **Environment overrides**: `QWEN_API_KEY`, `QWEN_API_BASE`, `QWEN_MODEL`

### Workspace files

- **Bootstrap docs**: `AGENTS.md`, `SOUL.md`, `USER.md`, `TOOLS.md`, `IDENTITY.md` (optional)
- **Memory**: `memory/MEMORY.md` (long-term), `memory/YYYY-MM-DD.md` (daily notes)
- **Skills**: `skills/{name}/SKILL.md` per skill
- **Heartbeat**: `HEARTBEAT.md`

### Retention / archival

**Not evidenced.** Sessions grow unbounded. The `LastConsolidated` field suggests planned consolidation but no consolidation logic is implemented.

---

## 10. Security and Access Control

### Authentication model

**None.** The system is single-user with no authentication. The `.env` file and `config.json` contain API keys in plaintext.

### Authorization model

**None.** All CLI commands are available to whoever can execute the binary.

### Sensitive data handling

- **Observed risk**: API keys are stored in `.env` and `config.json` in the workspace. Both are listed in `.gitignore`.
- The `exec` tool allows arbitrary shell command execution with a configurable timeout.
- File tools are workspace-relative but path traversal prevention is **not evidenced** in the current code.

### Transport security

The Qwen provider uses HTTPS (`https://dashscope.aliyuncs.com`). Web fetch uses `go-readability` for HTML extraction. No TLS configuration is exposed.

---

## 11. Configuration, Environment, and Deployment Hints

### Environment model

Single local environment. No staging/production distinction.

### Config sources (in priority order)

1. Environment variables (`QWEN_API_KEY`, `QWEN_API_BASE`, `QWEN_MODEL`) — override config file values
2. `.env` file loaded via `godotenv`
3. `{workspace}/config.json` — JSON config file
4. Hardcoded defaults in `DefaultConfig()`

### Default configuration values

| Setting | Default |
|---------|---------|
| Model | `qwen3-max` |
| Max tokens | 8192 |
| Temperature | 0.7 |
| Max tool iterations | 20 |
| Web search max results | 10 |
| Web fetch max chars | 50000 |
| Exec timeout | 10 seconds |
| Heartbeat enabled | true |
| Heartbeat interval | 60 seconds |
| Console channel enabled | true |
| Console prompt | `You>` |

### Deployment hints

- Single Go binary, no containers evidenced
- Workspace defaults to `.tinybot/workspace` relative to the current working directory
- `go.mod` requires Go 1.25.5
- Direct dependency on `github.com/openai/openai-go` for LLM API calls
- `.env` file is loaded from the working directory

---

## 12. Observability and Operations

### Logging

**Minimal.** One commented-out `slog.Info` call in `file_session_repo.go`. `AGENTS.md` mentions OpenTelemetry instrumentation as a goal but no spans or metrics are implemented.

### Metrics

**None evidenced.**

### Health checks

The `status` command provides a basic health check by verifying workspace file existence. The heartbeat service provides a periodic self-check mechanism via `HEARTBEAT.md`.

### Monitoring

**None evidenced.** Background runtime errors (cron, heartbeat) are silently swallowed in the ticker loops.

---

## 13. Testing and Verification Strategy

### Unit testing patterns

- **Table-driven tests** are used extensively (observed in session, message, cron, tool, config, run, and service tests).
- **Fake/stub implementations** for all service-layer interfaces: `fakeSessionRepository`, `fakeLLMClient`, `fakeToolRegistry`, `fakeHeartbeatAgent`, `fakeCronRepo`, etc.
- **Temp directories** via `t.TempDir()` for all file-system tests, avoiding real workspace pollution.
- **Context propagation testing**: tests verify that context values are passed through to repository methods.

### Integration testing patterns

- `TestGatewayApp_Run_ConsoleRoundTrip` is a near-integration test: it wires real bus, channel, loop, and fake LLM/tools, then verifies end-to-end message flow through the gateway.
- `TestService_ProcessMessage_ToolLoopRegression` verifies the full tool-call loop with fake LLM sequences.

### Test coverage areas

| Package | Tests | Focus |
|---------|-------|-------|
| `cmd/tinybot` | 7 tests | CLI dispatch, onboard, status, direct chat, cron commands |
| `internal/app` | 6 tests | Config loading, onboard, status, paths, gateway round-trip |
| `internal/domain/model` | 5 tests | Session, message, LLM response, cron job |
| `internal/service/chat` | 10+ tests | Service construction, message processing, tool loop, prompt building |
| `internal/service/cron` | 3 tests | Service construction, trigger once, error recording |
| `internal/service/heartbeat` | 4 tests | Trigger skipping, actionability, agent invocation |
| `internal/repository/sessionrepo` | 5 tests | Save, load, get-or-create, list, invalidate |
| `internal/repository/cronrepo` | 4 tests | List, save, empty file, invalid JSON |
| `internal/transport/bus` | 4 tests | Round-trip, close, context cancellation |
| `internal/transport/channel` | 4 tests | Console publish, send, empty messages, manager dispatch |
| `internal/transport/gateway` | 2 tests | Forward, fallback on error |
| `internal/transport/runtime` | 6 tests | Cron/heartbeat construction, trigger, error swallowing |
| `internal/adapters/tool` | Multiple | Exec, read_file, write_file, list_dir, web_search, web_fetch |
| `internal/adapters/workspace` | 6+ tests | Memory store, skill meta, skills loader, bootstrap reader |

### Contract testing

Not present as a distinct category, but the fake implementations effectively serve as contract tests for service-layer interfaces.

### Backward compatibility checks

No automated backward compatibility checks. The porting notes in `docs/porting-notes/` document intentional behavioral differences from Python nanobot.

---

## 14. Compatibility-Sensitive Areas for v2

### Must preserve

1. **Session JSONL format**: The metadata line + message lines structure is the durable storage format. Any change requires migration logic.
2. **Cron jobs.json format**: `CronJob` JSON structure with all fields. Existing job files must remain loadable.
3. **Session key convention**: `{channel}:{chatID}` pattern is embedded in session file paths and used as the primary lookup key.
4. **CLI command signatures**: `tinybot <message>`, `tinybot gateway`, `tinybot onboard`, `tinybot status`, `tinybot cron {list|add|remove|run-once}` — users depend on these.
5. **Workspace directory layout**: `.tinybot/workspace/` with `config.json`, `memory/MEMORY.md`, `skills/`, `sessions/`, `cron/`, `HEARTBEAT.md`, and bootstrap files.
6. **Environment variable names**: `QWEN_API_KEY`, `QWEN_API_BASE`, `QWEN_MODEL`.
7. **Tool execution behavior**: File tools use workspace-relative paths. The exec tool has a configurable timeout. Tool results are strings.
8. **Prompt assembly order**: identity → bootstrap docs → memory → always skills → skill summary. Tests assert this order.
9. **Fallback messages**: "Sorry, I encountered an error calling the AI model." for empty LLM responses. "Sorry, I encountered an error: {err}" for gateway processor failures.
10. **Heartbeat gating logic**: `IsHeartbeatEmpty` rules (headers, comments, empty checkboxes are non-actionable).

### Likely safe to improve

1. **LLM provider abstraction**: Currently hardcoded to Qwen. Adding multi-provider support is explicitly a TODO.
2. **Config structure**: Adding new sections or fields is safe as long as defaults are provided and existing fields are preserved.
3. **Tool registry design**: Adding new tools or improving the registry pattern is safe.
4. **Skill metadata parsing**: The front-matter parser is described as a known improvement area.
5. **Error messages**: Non-contractual error strings can be improved.
6. **In-memory bus buffer size**: Currently hardcoded to 16; making it configurable is safe.
7. **Session consolidation**: The `LastConsolidated` field exists but no consolidation logic is implemented. Adding it is safe.
8. **Logging and observability**: Adding structured logging or OpenTelemetry would be a pure addition.

### Uncertain / needs stakeholder confirmation

1. **Path traversal security for file tools**: No validation prevents tools from reading/writing outside the workspace. A v2 should decide whether to sandbox.
2. **Cron schedule types**: Only `every` is supported. Whether `at` and `cron` expression support should match Python nanobot needs confirmation.
3. **Session storage scaling**: JSONL files grow unbounded. Whether to add consolidation, pruning, or database backing needs decision.
4. **Multi-channel support scope**: Which channels (Telegram, WhatsApp, Discord, Slack) are priorities for Go implementation.
5. **Default workspace location**: Currently project-local (`.tinybot/workspace`). Python nanobot uses `~/.nanobot/`. Whether to switch to a home-directory default needs decision.

---

## 15. Risks, Gaps, and Open Questions

### Critical gaps

1. **No path traversal protection on file tools**: The exec, read_file, write_file, edit_file, and list_dir tools resolve paths relative to the workspace but do not prevent `../` traversal. This is a security risk for any deployment beyond personal use.
2. **API keys in plaintext**: `.env` and `config.json` store API keys without encryption. While `.gitignore` excludes them, there's no runtime protection.
3. **No graceful error recovery in tool loop**: If the LLM continuously requests tools for 20 iterations, the user gets a generic fallback message with no indication of what happened.

### Architectural uncertainties

4. **Legacy package cleanup**: `internal/ports/` and `internal/usecase/` are being deleted in the current working tree but the git status shows them as pending changes. The migration to `internal/service/` and `internal/transport/` is in progress.
5. **Progressive skill activation**: Method signatures accept skill names but the implementation ignores them, always using only always-on skills and summaries.
6. **Session consolidation**: The `LastConsolidated` field and `GetHistory` trimming suggest a planned consolidation feature that doesn't exist yet.

### Operational concerns

7. **Silent error swallowing**: Cron and heartbeat runners swallow trigger errors. In a production setting, this would hide failures.
8. **No request ID or trace context**: Despite `AGENTS.md` calling for OpenTelemetry, no instrumentation exists.
9. **Unbounded session growth**: Sessions accumulate messages forever with no pruning or archival.

### Missing from repository

10. **No CI/CD configuration** (no Makefile, no GitHub Actions, no Dockerfile).
11. **No README.md** for the Go project itself (only the Python `nanobot/README.md` exists).
12. **No integration tests against a real LLM** (all LLM interactions are faked in tests).
13. **No load or concurrency testing** for the gateway mode.

---

## 16. Appendix: Evidence Map

| Conclusion | Key evidence files |
|------------|-------------------|
| CLI command dispatch | `cmd/tinybot/main.go`, `cmd/tinybot/run.go` |
| Chat service orchestration | `internal/service/chat/service.go` |
| Prompt assembly order | `internal/service/chat/prompt_builder.go` |
| Tool definitions and execution | `internal/adapters/tool/*.go`, `internal/adapters/tool/contracts.go` |
| Session JSONL format | `internal/repository/sessionrepo/file_session_repo.go` |
| Cron job persistence | `internal/repository/cronrepo/file_cron_repo.go` |
| Domain models | `internal/domain/model/session.go`, `internal/domain/model/message.go`, `internal/domain/model/llm.go`, `internal/domain/model/cron.go` |
| Gateway wiring | `internal/app/gateway.go` |
| Transport contracts | `internal/transport/contracts.go` |
| Bus implementation | `internal/transport/bus/memory_bus.go` |
| Console channel | `internal/transport/channel/console.go` |
| Gateway loop | `internal/transport/gateway/loop.go` |
| Cron/heartbeat runtime | `internal/transport/runtime/cron.go`, `internal/transport/runtime/heartbeat.go` |
| Cron service logic | `internal/service/cron/service.go` |
| Heartbeat service logic | `internal/service/heartbeat/service.go` |
| Config model and loading | `internal/app/config.go` |
| Onboard flow | `internal/app/onboard.go` |
| Status reporting | `internal/app/status.go` |
| Workspace bootstrap reader | `internal/adapters/workspace/bootstrap_reader.go` |
| Memory store | `internal/adapters/workspace/memory_store.go` |
| Skills loader and metadata | `internal/adapters/workspace/skills_loader.go`, `internal/adapters/workspace/skill_meta.go` |
| LLM provider (Qwen) | `internal/adapters/provider/qwen.go` |
| Architecture decisions | `AGENTS.md`, `specs.md`, `docs/porting-notes/*.md` |
| Python nanobot reference | `nanobot/README.md` |
| Dependency list | `go.mod` |
