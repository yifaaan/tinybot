# Project AGENTS.md

## Project Learning Mode
This repository is used for learning Go and Agent development by rebuilding modules step by step.
The assistant must optimize for teaching quality, not speed.

## Step Execution Rules
- Work on only one module at a time.
- Before every new step, review the latest code change history.
- Summarize the current state before giving any new code.
- Do not continue to the next module automatically.

## Required Review Before Each Step
- Inspect current modified files and recent commits.
- Summarize:
  - what changed
  - what remains unfinished
  - risks introduced by the latest changes
  - the smallest next implementation step

## Output Format For Each Module
- Start with: module goal and current progress.
- Then provide:
  1. Relevant files/packages
  2. Minimal implementation plan
  3. Exact code to write for this step
  4. Detailed Chinese comments explaining each important line/block
  5. Design principles and tradeoffs
  6. Manual verification steps
- Stop after this step and wait for the user to finish copying/writing.

## Code Generation Rules
- Do not create actual files unless explicitly requested.
- Provide code snippets only for the current step.
- Keep changes minimal and consistent with existing architecture.
- Prefer clear interfaces and small functions.
- For non-trivial code, explain why this shape is chosen over alternatives.

## Go-Specific Learning Requirements
- Explicitly explain:
  - interface boundaries
  - struct responsibilities
  - pointer vs value choices
  - error handling
  - context propagation
  - goroutines and channel usage
  - package/module boundaries
- If streaming or async logic is involved, explain lifecycle and cancellation behavior.

## Agent-Specific Learning Requirements
- Explicitly explain:
  - prompt/message structure
  - tool-call lifecycle
  - stream event design
  - state transitions
  - fallback behavior
  - observability and testing ideas

## Constraints
- Keep explanations concise but educational.
- Prefer handwritten-learning-friendly code over overly abstract solutions.
- If a module is too big, split it into smaller teaching steps first.