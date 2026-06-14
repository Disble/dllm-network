# Ollama Telemetry

Passive-only Windows tray app for local Ollama telemetry with a Wails + React dashboard.

## Quick path

1. Install frontend dependencies with `bun install` inside `frontend/`.
2. Run Go validation with `bun run test:go`.
3. Run full local validation with `bun run validate`.
4. Run the frontend React review pass with `bun run doctor:react`.

## What the dashboard shows

- Confirmed telemetry from passive local sources such as Ollama API polling, process state, loopback ownership, and host metrics.
- Inferred activity derived from passive signals, always labeled with confidence and evidence.
- Passive limitations that explain which exact request metrics are unavailable.

The dashboard keeps confirmed telemetry and inferred activity in separate sections.

## Honest product boundary

This product stays passive-only. It does not proxy traffic, change client endpoints, inject SDKs, wrap requests, or require connection-path changes.

Exact request latency, exact token counts, exact HTTP status codes, exact request or response payloads, and exact streaming chunks are unavailable in passive mode.

See `docs/passive-telemetry.md` for confirmed-versus-inferred semantics and `docs/development.md` for local commands.
