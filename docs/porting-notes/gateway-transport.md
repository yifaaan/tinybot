# Gateway Transport Porting Notes

This note tracks the transport-boundary refactor for the gateway runtime.

## Scope

- Move the gateway loop, in-memory bus, and channel runtime out of `internal/usecase` and `internal/adapters`.
- Keep `internal/app/gateway.go` as the composition root that wires chat service, transport bus, channels, cron, and heartbeat together.
- Preserve existing behavior for the console gateway path.

## New transport layout

- `internal/transport/contracts.go` defines the shared transport contracts: `MessageBus`, `Channel`, `MessageProcessor`, and `ErrBusClosed`.
- `internal/transport/bus` contains the in-memory bus implementation used by the local gateway runtime.
- `internal/transport/gateway` contains the loop that bridges inbound bus traffic to the chat service.
- `internal/transport/channel` contains channel runtime code, including the channel manager and console channel.

## Compatibility decisions

1. The gateway loop still publishes a fallback outbound message when the chat service returns an error.
2. The console channel still defaults to `cli` traffic and uses the same prompt/rendering behavior as before.
3. The in-memory bus remains queue-based and shutdown-safe, now using `transport.ErrBusClosed` instead of the old `ports` sentinel.

## Known follow-up work

- Split concrete external chat integrations such as Telegram and WhatsApp into their own transport subpackages once they exist in Go.
- Decide whether `internal/app/gateway.go` should later move into an explicit `internal/transport/gatewayapp` package or remain the composition root.
