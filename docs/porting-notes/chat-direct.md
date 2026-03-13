# Chat Direct Porting Notes

This note tracks the first Go rewrite milestone for the direct chat path.

## Scope

- Migrate the direct chat orchestration from `internal/usecase/chat` to `internal/service/chat`.
- Keep `cmd/tinybot/run.go -> internal/app/bootstrap.go -> chat service` as the active path.
- Do not move gateway, bus, channel, cron, or heartbeat into new transport packages in this milestone.

## Responsibilities and boundaries

- `internal/service/chat/service.go` owns the chat turn orchestration: session load, prompt assembly, LLM call, tool loop, and session persistence.
- `internal/service/chat/prompt_builder.go` owns prompt assembly only: identity, bootstrap docs, memory, always-on skills, and skill summary.
- `internal/adapters/repository`, `internal/adapters/provider`, `internal/adapters/tool`, and `internal/adapters/workspace` remain the concrete infrastructure layer for now.

## Compatibility decisions

1. Keep Go's explicit `ProcessDirect(ctx, sessionKey, content)` behavior. This is intentionally different from Python nanobot so CLI calls can control thread boundaries.
2. Keep the current JSONL session trace shape for user, assistant, and tool messages.
3. Treat missing optional workspace files (`IDENTITY.md`, memory notes, skills) as soft failures during prompt assembly.

## Known differences from Python nanobot

- The Go direct path preserves the explicit session key override instead of deriving the thread strictly from channel/chat ID.
- This milestone does not route direct chat through bus/channel/gateway layers.
- Selected non-always skills are still summarized instead of being eagerly loaded into the prompt; explicit progressive activation remains a later TODO.
