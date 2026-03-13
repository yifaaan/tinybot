# Contracts Cleanup Notes

This note tracks the removal of the old shared `internal/ports` package from the active runtime path.

## Scope

- Replace `internal/ports/llm.go`, `internal/ports/session.go`, and `internal/ports/tool.go` with service-local or adapter-local contracts.
- Stop importing `internal/ports` from the active app, CLI, service, transport, and built-in tool paths.
- Keep concrete adapters implementing the new narrow contracts without changing observable behavior.

## Contract ownership after cleanup

- `internal/service/chat/contracts.go` now owns the chat-facing `SessionRepository`, `CompletionClient`, and `ToolExecutor` boundaries.
- `internal/service/cron/contracts.go` owns the cron repository boundary for the active runtime path.
- `internal/adapters/tool/contracts.go` owns concrete built-in tool registration and schema contracts.

## Compatibility decisions

1. The concrete Qwen provider still returns the same normalized chat response shape; only the contract package changed.
2. The file session repository still uses the same JSONL layout; only the consumer interface changed.
3. Built-in tool schemas and execution behavior are unchanged; only the spec/interface type moved from `internal/ports` into the tool adapter package.
