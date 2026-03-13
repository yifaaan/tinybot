# Repository Boundary Notes

This note closes the repository side of the current `domain / service / repository / transport` refactor.

## Current ownership

### `internal/repository/sessionrepo`

Used by:

- `internal/service/chat`
- `internal/app/bootstrap.go` as composition root wiring

Owns:

- session JSONL file layout
- session cache
- session list/read/write helpers

Does not own:

- prompt assembly
- tool execution
- LLM calls
- cron or heartbeat scheduling

Reference module alignment:

- Python source: `nanobot/nanobot/session/manager.py`
- Matching behavior: load-or-create session, cache hot sessions, persist conversation trace
- Intentional Go difference: the current Go rewrite preserves tool traces in session history, not just final user/assistant turns

### `internal/repository/cronrepo`

Used by:

- `internal/service/cron`
- `cmd/tinybot/run.go` for `cron list/add/remove/run-once`
- `internal/app/gateway.go` as composition root wiring

Owns:

- `cron/jobs.json` file path
- full job slice load/save
- repository-edge validation of `model.CronJob`

Does not own:

- ticker loops
- next polling interval
- calling the chat service
- CLI formatting

Reference module alignment:

- Python source: `nanobot/nanobot/cron/service.py`
- Matching behavior: durable scheduled-job store and persisted run state
- Current Go gap: Python nanobot supports richer schedules (`at`, `cron`, optional delivery payloads), while the active Go path still centers on `every`

## Service to repository map

Use this as the mental model for the current codebase:

```text
service/chat      -> repository/sessionrepo
service/cron      -> repository/cronrepo
service/heartbeat -> no dedicated repository yet; reads HEARTBEAT.md directly
transport/*       -> no repository; only runtime/message delivery concerns
```

## Suggested beginner-friendly next steps

1. Keep `sessionrepo` focused on chat persistence only.
2. Keep `cronrepo` focused on scheduled-job persistence only.
3. If heartbeat later needs durable state, add a new repository package instead of overloading `cronrepo`.
4. If session storage grows more complex, add `context.Context` to the chat-facing repository contract before changing the backend format.

## Skeleton direction for future cleanup

```go
package chat

type SessionRepository interface {
    GetOrCreateSession(key string) *model.Session
    SaveSession(session *model.Session) error
}
```

```go
package cron

type Repository interface {
    ListJobs(ctx context.Context) ([]model.CronJob, error)
    SaveJobs(jobs []model.CronJob) error
}
```

The important part is not the file format. The important part is that each interface matches one service's real needs.
