package heartbeat

import "context"

// AgentTurner runs one direct chat turn on behalf of the heartbeat service.
//
// Inputs: a session key and the fixed heartbeat prompt.
// Outputs: the reply text and a direct-turn error.
// State changes: delegated to the chat service implementation.
// Side effects: delegated to the chat service implementation, such as model calls or session writes.
// Compatibility: preserves the current heartbeat-to-chat integration contract.
type AgentTurner interface {
	ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error)
}
