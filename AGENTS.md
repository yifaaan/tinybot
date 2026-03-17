# Repository Guidelines

## Project Structure & Module Organization
`cmd/tinybot` contains the CLI entrypoint. Core business logic lives under `internal/`: `app` bootstraps config and dependencies, `service` orchestrates chat and scheduling flows, `repository` persists sessions and cron data, `transport` hosts channel and gateway wiring, and `desktop` exposes the Wails-facing desktop service. The desktop shell lives in `frontend/`, with app-level wiring in `frontend/src/app`, CherryStudio-style panels in `frontend/src/features`, and generated Wails runtime files in `frontend/wailsjs`. Built desktop binaries land in `build/bin`.

## Build, Test, and Development Commands
Use `go build ./cmd/tinybot` to build the CLI. Run `go test ./...` for the full Go test suite, or narrow scope with `go test ./internal/desktop ./internal/repository/sessionrepo`. In `frontend`, use `npm install` once and `npm run build` to produce `frontend/dist`. For the desktop app, use `wails build -tags desktop -m`. The `-m` flag matters here because Wails binding generation should skip `go mod tidy` when `frontend/node_modules` exists.

## Coding Style & Naming Conventions
Format Go code with `gofmt`; keep exported identifiers in `CamelCase` and package-local helpers in `camelCase`. Follow the existing Go package boundaries instead of introducing cross-layer shortcuts. Frontend code uses strict TypeScript, React function components, and 2-space indentation. Keep new UI modules small and feature-scoped under `frontend/src/features/<area>`. Prefer black/white theme tokens over ad hoc colors.

## Testing Guidelines
Place Go tests next to the code they cover in `_test.go` files. Favor focused table-driven tests for service and repository logic. When touching desktop chat flows, cover both happy-path streaming and failure cases. For frontend changes, at minimum run `npm run build`; for desktop integration changes, also run `wails build -tags desktop -m`.

## Commit & Pull Request Guidelines
Recent history follows conventional prefixes such as `feat:`, `fix:`, `refactor:`, and `docs:`. Keep commit subjects imperative and scoped to one change. PRs should include a short summary, impacted areas, commands run for verification, and screenshots for desktop UI changes. Call out config, provider, or workspace assumptions explicitly.
