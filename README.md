# Ollama Telemetry

> A Chrome DevTools "Network tab" for your local [Ollama](https://ollama.com) —
> plus an MCP server that lets LLMs query the traffic it captures.

Ollama Telemetry is a Windows tray app (Wails + React) that **passively
observes** the HTTP traffic between your tools and a local Ollama instance and
presents it like a request inspector: models, endpoints, status codes,
tokens/sec, latency, and full request/response bodies. It then exposes that same
captured data to any MCP client (Claude Desktop, Claude Code, …) through a
built-in **Model Context Protocol server**, so an LLM can answer questions about
your own inference activity.

Passive means it **does not proxy traffic, change endpoints, inject SDKs, or
wrap requests**. It observes via packet inspection (WinDivert); your apps talk to
Ollama exactly as before.

## Features

- **Live request inspector** — every captured Ollama exchange with method,
  endpoint, status code, streaming flag, prompt/response bodies, and headers.
- **Real performance metrics** — token counts, tokens/sec, and latency derived
  from captured responses (not estimated).
- **MCP server** — query your telemetry from an LLM through a compact
  Context7-style flow: `resolve_inference_context`, `search_inferences`, and
  `get_inference_context`. See [`docs/mcp.md`](docs/mcp.md).
- **Session-scoped by design** — like DevTools' Network tab, data is a working
  view of the current session, not a permanent archive.
- **Honest about limits** — when run without elevation it degrades to passive
  API polling and clearly labels confirmed vs. inferred signals.

## Requirements

- **Windows** (capture uses WinDivert; the app is a Windows tray app).
- **Administrator** for per-request packet capture (WinDivert). Without it, the
  app falls back to passive Ollama API polling.
- A local **Ollama** instance (default `http://127.0.0.1:11434`).
- **Go 1.26+**, **[Bun](https://bun.sh)**, and the
  **[Wails v2 CLI](https://wails.io)** to build from source:
  `go install github.com/wailsapp/wails/v2/cmd/wails@latest`.

## Quick start

```sh
# 1. Install frontend dependencies
cd frontend && bun install && cd ..

# 2. Run the GUI app in dev mode (run your terminal as Administrator
#    so WinDivert capture is active)
wails dev

# 3. Build the MCP sidecar binary
go build -o ollama-telemetry-mcp.exe ./cmd/ollama-telemetry-mcp
```

Generate some Ollama traffic, watch it appear in the dashboard, then wire the
sidecar into your MCP client — full walkthrough in [`docs/mcp.md`](docs/mcp.md).

To produce a release build of the GUI app: `wails build` (output under
`build/bin/`).

## Using the MCP server

Build the sidecar (above), then register it with your MCP client by absolute
path — for example, Claude Desktop's `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "ollama-telemetry": {
      "command": "C:\\absolute\\path\\to\\ollama-telemetry-mcp.exe"
    }
  }
}
```

The GUI app must have run at least once (the sidecar reads its database
read-only and never creates it). Tools, example prompts, and troubleshooting:
**[`docs/mcp.md`](docs/mcp.md)**.

## Documentation

| Doc | What's in it |
|-----|--------------|
| [`docs/mcp.md`](docs/mcp.md) | Build, register, and use the MCP server; 3-tool reference; troubleshooting. |
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | How capture, persistence, the read ports, and the MCP core fit together; the enforced architecture boundaries. |
| [`docs/development.md`](docs/development.md) | Local validation commands (`bun run validate`, tests, lint, typecheck) and conventions. |
| [`docs/passive-telemetry.md`](docs/passive-telemetry.md) | Confirmed-versus-inferred semantics of the passive polling layer and its limitations. |

## Development

Validation runs from the repository root via Bun:

```sh
bun run validate     # Go tests + Go lint + frontend tests, lint, typecheck, React Doctor
bun run test:go      # Go tests only
bun run lint:go      # golangci-lint (enforces architecture boundaries)
```

Architecture boundaries are **machine-enforced** by `golangci-lint`/`depguard`
(domain purity, SDK isolation, read-side/write-side separation) — see
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md). Full command reference:
[`docs/development.md`](docs/development.md).

## Project layout

```
internal/capture       Packet capture (WinDivert), HTTP reassembly + parsing
internal/telemetry     Domain types (driver-free inference model), polling
internal/store         InferenceReader/InferenceWriter ports
internal/store/sqlite  Durable WAL SQLite store (the cross-process seam)
internal/persistence   Async bus subscriber → batched, bounded writes
internal/mcp           MCP server core: 3-tool MCP contract (SDK quarantined here)
internal/app           Wails app lifecycle, dashboard wiring
cmd/ollama-telemetry-mcp   stdio MCP sidecar binary
frontend               React + Vite dashboard
```

## Contributing

Issues and pull requests are welcome. Before submitting:

1. Run `bun run validate` and make sure it is green.
2. Keep changes within the architecture boundaries enforced by `bun run lint:go`
   (see [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)).
3. Use [Conventional Commits](https://www.conventionalcommits.org/) for commit
   messages.

## License

Licensed under the **Apache License 2.0** — see [`LICENSE`](LICENSE).

Copyright 2026 disble.
