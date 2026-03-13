// Package sessionrepo contains file-backed repositories used by service/chat.
//
// It is responsible for chat-session persistence only:
//   - load or create a conversation session by stable session key
//   - persist the session trace as JSONL under the workspace
//   - expose small helper methods for local CLI or status tooling
//
// It should not own chat orchestration, prompt building, or cron scheduling.
package sessionrepo
