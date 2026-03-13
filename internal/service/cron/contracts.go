package cron

import (
	"context"

	"tinybot/internal/domain/model"
)

// Repository stores and retrieves scheduled cron jobs for the cron service.
//
// Responsibilities:
//   - list the persisted cron jobs that should be evaluated
//   - save the updated job state after a trigger pass
//
// Inputs: contexts for reads and a full job slice for writes.
// Outputs: validated cron jobs or persistence errors.
// State changes: concrete implementations usually rewrite the on-disk cron job store.
// Side effects: file or database I/O in concrete repositories.
// Compatibility: keeps the current Go JSON-backed cron store behavior while narrowing the service boundary.
type Repository interface {
	ListJobs(ctx context.Context) ([]model.CronJob, error)
	SaveJobs(ctx context.Context, jobs []model.CronJob) error
}

// AgentTurner runs one direct chat turn on behalf of a cron job.
//
// Inputs: a stable session key and the job prompt.
// Outputs: the model reply text and an execution error when the turn fails.
// State changes: delegated to the chat service implementation.
// Side effects: delegated to the chat service implementation, such as model calls or session writes.
// Compatibility: preserves the current cron-to-chat integration contract.
type AgentTurner interface {
	ProcessDirect(ctx context.Context, sessionKey string, content string) (string, error)
}
