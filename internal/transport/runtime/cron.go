package runtime

import (
	"context"
	"errors"
	"time"
)

// CronRunner periodically triggers the cron service.
//
// Responsibilities:
//   - own the ticker interval for cron evaluation
//   - call the cron service on each tick until shutdown
//
// Inputs: a cron service and an interval in seconds.
// Outputs: runtime errors only when configuration is invalid; service trigger errors are swallowed for now to preserve current behavior.
// State changes: ticker lifecycle only.
// Side effects: periodic wakeups and delegated service calls.
// Compatibility: preserves the current Go cron Run loop while moving the scheduling concern out of the service package.
type CronRunner struct {
	service  cronTriggerer
	interval time.Duration
}

type cronTriggerer interface {
	TriggerOnce(ctx context.Context) (string, error)
}

// NewCronRunner constructs a periodic cron runtime.
func NewCronRunner(service cronTriggerer, intervalSeconds int) (*CronRunner, error) {
	if service == nil {
		return nil, errors.New("cron runtime: service is required")
	}
	if intervalSeconds <= 0 {
		return nil, errors.New("cron runtime: intervalSeconds must be positive")
	}
	return &CronRunner{
		service:  service,
		interval: time.Duration(intervalSeconds) * time.Second,
	}, nil
}

// Run triggers the cron service until the context is canceled.
func (r *CronRunner) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := r.service.TriggerOnce(ctx); err != nil {
				continue
			}
		}
	}
}
