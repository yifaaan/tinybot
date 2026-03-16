# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**tinybot** is a Go rewrite of the Python [nanobot](https://github.com/HKUDS/nanobot) project — an ultra-lightweight personal AI assistant that combines LLM-powered chat with tool execution, scheduled tasks, persistent memory, and multi-channel messaging.

The system receives natural-language messages from a user (via CLI or gateway console channel), assembles a context-aware prompt, sends it to an LLM provider, executes any tool calls the model requests, and returns the final reply. Conversations are persisted as JSONL files.

## Build and Test Commands

```bash
# Build the binary
go build -o tinybot.exe ./cmd/tinybot

# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/service/chat/...

# Run a specific test with verbose output
go test -v -run TestService_ProcessMessage ./internal/service/chat/...

# Run tests with coverage
go test -cover ./...
```

## CLI Commands

```bash
# Direct chat (any non-command input is treated as a chat message)
tinybot <message>

# Initialize workspace with default files
tinybot onboard

# Check workspace status
tinybot status

# Start long-running gateway mode (console channel)
tinybot gateway

# Cron job management
tinybot cron list
tinybot cron add <name> <every_seconds> <prompt>
tinybot cron add-cron <name> <cron_expr> <prompt>  # e.g., "0 9 * * *"
tinybot cron add-at <name> <rfc3339_time> <prompt>
tinybot cron remove <job_id>
tinybot cron run-once
```

## Architecture Overview

### Layered Structure

```
cmd/tinybot/           # CLI entrypoint and command dispatch
internal/
  app/                 # Composition root: wires all dependencies together
  domain/model/        # Core domain types: Session, Message, LLMResponse, ToolCall, CronJob
  service/
    chat/              # Chat turn orchestration (core agent loop)
    cron/              # Cron trigger-once evaluation
    heartbeat/         # Heartbeat file evaluation
  repository/
    sessionrepo/       # JSONL session persistence
    cronrepo/          # JSON cron job persistence
  transport/
    bus/               # In-memory message bus
    channel/           # Console channel and channel manager
    gateway/           # Gateway loop bridging bus to chat service
    runtime/           # Periodic ticker runners for cron and heartbeat
  adapters/
    provider/          # LLM provider adapters (Qwen/OpenAI-compatible)
    tool/              # Built-in tool implementations and registry
    workspace/         # Bootstrap reader, memory store, skills loader
```

### Key Dependencies

- `github.com/openai/openai-go` — LLM provider SDK (OpenAI-compatible API calls)
- `github.com/robfig/cron/v3` — Cron expression parsing and validation
- `github.com/go-shiori/go-readability` — Web content extraction for web_fetch tool
- `github.com/joho/godotenv` — .env file loading

### Composition Root (internal/app/)

All dependency wiring happens in `bootstrap.go` via `NewApp()` and `NewGatewayApp()`:

1. `NewApp()` — creates the core chat pipeline: config → provider → tool registry → session repo → consolidator → prompt builder → chat service
2. `NewGatewayApp()` — extends `NewApp()` with transport: message bus → gateway loop → channel manager → cron runner → heartbeat runner

The `run()` function in `cmd/tinybot/run.go` is separated from `main()` for testability — tests inject factory functions (e.g., `newDirectChatProcessor`) to stub out dependencies.

### Core Service Contracts (internal/service/chat/contracts.go)

- **SessionRepository**: `GetOrCreateSession`, `SaveSession`
- **CompletionClient**: `Chat(ctx, messages, tools, maxTokens, temperature)`
- **ToolExecutor**: `GetDefinitions`, `Execute(ctx, name, params)` + `SetMessageContext(channel, chatID)` for tool context propagation
- **PromptBuilder**: `BuildMessages`, `AddAssistantMessage`, `AddToolResult`

### Chat Service Turn Flow (internal/service/chat/service.go)

1. Load or create session
2. Build prompt from history + current message + workspace context
3. Call LLM
4. If tool calls: execute tools, append results, repeat from step 3 (up to `max_tool_iterations`)
5. Save session trace
6. Return reply

Before the tool loop, the service calls `SetMessageContext(channel, chatID)` on the tool executor so the `message` tool knows which channel to target.

### Transport and Gateway Concurrency

**Contracts** (internal/transport/contracts.go):
- **MessageBus**: In-process pub/sub for inbound/outbound messages
- **MessageProcessor**: Single-message handler (bridges gateway to chat service)
- **Channel**: External chat transport (console, telegram, etc.)

**Goroutine model in gateway mode** — all coordinated by context cancellation:
- Gateway loop: blocks on bus, calls chat service, publishes reply
- Each channel: blocks on its input source (stdin, Telegram API, etc.)
- Cron runner: blocks on ticker, evaluates due jobs
- Heartbeat runner: blocks on ticker, evaluates HEARTBEAT.md

### Tool Registry (internal/adapters/tool/)

Tools implement the `Tool` interface:
- `Spec() ToolSpec` — returns OpenAI-compatible function schema
- `Execute(ctx, params) (string, error)` — executes the tool

Built-in tools: `exec`, `read_file`, `write_file`, `edit_file`, `list_dir`, `web_search`, `web_fetch`, `message`

### Cron Job Scheduling (internal/domain/model/cron.go)

Three schedule kinds:
- **Every** — repeats every N seconds
- **At** — one-time execution at RFC3339 timestamp
- **Cron** — standard cron expression (validated with robfig/cron/v3)

Jobs support optional **delivery targets** (channel + chatID) to dispatch execution results to specific channels via `ResultDispatcher`.

### Session Persistence (internal/repository/sessionrepo/)

JSONL format with a non-obvious convention: **the first line is session metadata** (key, timestamps, LastConsolidated), subsequent lines are messages (one per line).

```
{"key":"cli:direct","created_at":"...","updated_at":"...","last_consolidated":0}
{"role":"user","content":"Hello","created_at":"..."}
{"role":"assistant","content":"Hi!","created_at":"..."}
```

### Session Consolidation (internal/service/chat/consolidation.go)

When conversation history exceeds `token_limit`, old messages are summarized by the LLM into a single system message. `Session.LastConsolidated` tracks the cutoff index. `GetHistory()` returns only unconsolidated messages. The `keep_recent` config preserves the N most recent messages uncompressed.

## Configuration

- Config file: `{workspace}/config.json`
- Environment overrides: `QWEN_API_KEY`, `QWEN_API_BASE`, `QWEN_MODEL`
- Default workspace: `./workspace` (relative to working directory)

### Key Config Fields

```json
{
  "agents": {
    "max_tokens": 8192,
    "temperature": 0.7,
    "max_tool_iterations": 20,
    "consolidation": {
      "enabled": true,
      "token_limit": 60000,
      "keep_recent": 10
    }
  },
  "providers": {
    "active": "qwen",
    "list": {
      "qwen": {
        "kind": "qwen",
        "api_key": "...",
        "api_base": "https://dashscope.aliyuncs.com/compatible-mode/v1",
        "model": "qwen3-max"
      }
    }
  }
}
```

## Prompt Assembly Order (internal/service/chat/prompt_builder.go)

1. Identity template (hardcoded tinybot identity with current time and workspace path)
2. Bootstrap docs: AGENTS.md, SOUL.md, USER.md, TOOLS.md, IDENTITY.md (optional)
3. Memory: MEMORY.md and today's daily notes
4. Always-on skills: full body content of skills marked `always: true`
5. Skills summary: XML-formatted summary of all available skills

## Testing Patterns

- Table-driven tests with fake/stub implementations for all service-layer interfaces
- `t.TempDir()` for all file-system tests
- Tests verify context propagation through repository methods
- CLI tests use injectable factory functions to stub out the chat pipeline

## Session Key Convention

Format: `{channel}:{chatID}`
- CLI direct: `cli:direct`
- Gateway console: `console:gateway`
- Cron jobs: `cron:{job_id}`

## Key Behavioral Rules

- Tool loop iteration limit: 20 (configurable via `max_tool_iterations`)
- Missing optional files (bootstrap, memory, skills) do not abort prompt building
- Empty LLM response fallback: "Sorry, I encountered an error calling the AI model."
- Heartbeat only triggers LLM if HEARTBEAT.md contains actionable content (not just headers/comments/empty checkboxes)
