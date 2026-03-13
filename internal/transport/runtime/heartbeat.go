package runtime

import (
	"context"
	"errors"
	"time"
)

// HeartbeatRunner periodically triggers the heartbeat service.
//
// Responsibilities:
//   - own the heartbeat polling interval and enabled flag
//   - call the heartbeat service on each tick until shutdown
//
// Inputs: a heartbeat service, an interval in seconds, and whether the runtime is enabled.
// Outputs: runtime configuration errors only; trigger errors are swallowed to preserve current behavior.
// State changes: ticker lifecycle only.
// Side effects: periodic wakeups and delegated service calls.
// Compatibility: preserves the current Go heartbeat Run loop while moving the scheduling concern out of the service package.
type HeartbeatRunner struct {
	service  heartbeatTriggerer
	interval time.Duration
	enabled  bool
}

type heartbeatTriggerer interface {
	TriggerOnce(ctx context.Context) (string, error)
}

// NewHeartbeatRunner constructs a periodic heartbeat runtime.
func NewHeartbeatRunner(service heartbeatTriggerer, intervalSeconds int, enabled bool) (*HeartbeatRunner, error) {
	if service == nil {
		return nil, errors.New("heartbeat runtime: service is required")
	}
	if enabled && intervalSeconds <= 0 {
		return nil, errors.New("heartbeat runtime: intervalSeconds must be positive")
	}
	if intervalSeconds <= 0 {
		intervalSeconds = 1
	}
	return &HeartbeatRunner{
		service:  service,
		interval: time.Duration(intervalSeconds) * time.Second,
		enabled:  enabled,
	}, nil
}

// Run triggers the heartbeat service until the context is canceled.
func (r *HeartbeatRunner) Run(ctx context.Context) error {
	if !r.enabled {
		return nil
	}

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
