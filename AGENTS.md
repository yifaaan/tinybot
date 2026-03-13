# AGENTS.md
## Project Goal
This repository is a Go rewrite of the nanobot project for learning agent architecture, Go engineering, and clean backend design.
## Project Context
- The `nanobot/` directory contains the original simple implementation of the project and serves as a reference template.
- The current repository is a Go rewrite workspace based on that original implementation.
- One of the main goals is to improve the user's Go development skills during the rewrite.
- The user is a beginner in Go backend development.
## Project-Specific Expectations
- Prefer teaching-oriented guidance over fully automatic implementation.
- Before large code changes, explain:
  - what should be implemented next
  - why it should be implemented next
  - where it belongs in the project structure
- Prefer incremental milestones and minimal working skeletons.
- Avoid overengineering.
- When generating code, prefer skeletons with TODOs so the user can complete the implementation as practice.
## Engineering Priorities
- Prefer readability, explicit control flow, and testability over clever abstractions.
- Keep functions small and focused.
- Prefer interfaces at boundaries: model providers, tool execution, memory, transport, storage.
- Use dependency injection instead of globals.
- Always propagate `context.Context`.

## Architecture Rules
- Organize code with clear layers:
  - `cmd/` entrypoints
  - `internal/domain/` core domain models
  - `internal/service/` use cases and orchestration
  - `internal/repository/` persistence and provider adapters
  - `internal/transport/` HTTP, CLI, or chat adapters
  - `pkg/` reusable utilities only
- Keep framework code out of domain logic.
- Prefer composition over inheritance-like patterns.

## Go Conventions
- Run `go fmt ./...` and `go test ./...` after meaningful changes.
- Use wrapped errors: `fmt.Errorf("context: %w", err)`.
- Prefer table-driven tests.
- Add tests for exported behavior and regression fixes.
- Avoid hidden side effects.

## Agent-Specific Rules
- When porting a module from nanobot, first identify:
  - responsibilities
  - inputs and outputs
  - state transitions
  - external dependencies
  - retry/timeout behavior
- Preserve observable behavior before redesigning internals.
- When behavior is unclear, document assumptions in `docs/porting-notes/`.

## Observability
- New long-running or external-call paths should be instrumented with OpenTelemetry where practical.
- Include request IDs / trace context in logs when possible.

## Documentation
- Keep `AGENTS.md` short.
- Put detailed design notes in `docs/`.
- Add porting notes for each migrated subsystem.