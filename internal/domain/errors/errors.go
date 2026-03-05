package errors

import "errors"

// Session errors.
var (
	ErrSessionNotFound = errors.New("session not found")
)

// Tool errors.
var (
	ErrToolNotFound       = errors.New("tool not found")
	ErrMaxToolIterations  = errors.New("max tool iterations reached")
)

// LLM errors.
var (
	ErrLLMUnavailable = errors.New("llm provider unavailable")
	ErrLLMTimeout     = errors.New("llm request timed out")
)

// Channel errors.
var (
	ErrChannelClosed = errors.New("channel is closed")
	ErrSendFailed    = errors.New("failed to send message")
)

// Cron errors.
var (
	ErrJobNotFound      = errors.New("cron job not found")
	ErrInvalidSchedule  = errors.New("invalid cron schedule")
)
