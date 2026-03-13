# Background Runtime Porting Notes

This note tracks the split between service logic and periodic runtime logic for cron and heartbeat.

## Scope

- Move cron and heartbeat trigger logic into `internal/service`.
- Move periodic ticker-based scheduling into `internal/transport/runtime`.
- Keep `internal/app/gateway.go` as the composition root that wires chat, transport, cron, and heartbeat together.

## New structure

- `internal/service/cron` owns one trigger pass over due jobs and job-state updates.
- `internal/service/heartbeat` owns one evaluation pass over `HEARTBEAT.md`.
- `internal/transport/runtime/cron.go` owns the cron ticker loop.
- `internal/transport/runtime/heartbeat.go` owns the heartbeat ticker loop and enabled flag.

## Compatibility decisions

1. Cron and heartbeat runtimes still swallow trigger errors inside the ticker loop so the gateway keeps running.
2. `runCronRunOnce` now calls the cron service directly, without constructing a periodic runner.
3. Heartbeat still returns `HEARTBEAT_OK` when `HEARTBEAT.md` is missing or non-actionable.

## Follow-up work

- Decide whether background runtimes should eventually emit structured logs or OpenTelemetry spans for skipped and failed trigger passes.
- Revisit whether cron/heartbeat should share a common runtime abstraction once more background jobs exist.
