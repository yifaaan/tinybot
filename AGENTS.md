# AGENTS.md
## Project context
- `nanobot/` is the original simple version of the tinybot/nanobot project.
- The current repository is a Go rewrite workspace based on `nanobot/`.
- The main goal is not only to finish the rewrite, but also to improve the user's Go development skills step by step.
- The user is a beginner in Go backend development.
## Collaboration expectations
- Prefer teaching-oriented guidance over doing everything automatically.
- Before implementing large changes, first explain:
  - what to build next
  - why it should be built next
  - where it belongs in the project structure
- Prefer small, incremental milestones.
- Avoid overengineering.
- Keep architecture idiomatic, modular, and testable.
## Go architecture preferences
- Prefer a clean structure such as:
  - `cmd/`
  - `internal/`
  - `pkg/`
  - `configs/`
  - `test/`
- Use dependency injection instead of global state.
- Prefer interfaces at boundaries.
- Always propagate `context.Context`.
- Handle errors explicitly with wrapped errors.
- Keep functions small and focused.
## Output expectations
- If no project spec exists yet, create or update a root-level markdown spec file.
- When suggesting implementation steps:
  1. summarize what is already implemented
  2. identify the next best task for a beginner
  3. explain the purpose of the task
  4. provide skeleton code only, leaving TODOs for the user
- Respond in Chinese by default unless the user asks otherwise.