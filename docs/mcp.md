# Using the MCP server

Ollama Telemetry ships an **MCP (Model Context Protocol) server** that exposes
the inference requests it has captured to any MCP client — Claude Desktop,
Claude Code, or your own. It runs as a small **stdio sidecar binary**
(`cmd/ollama-telemetry-mcp`) that opens the same database the GUI app writes,
**read-only**, and serves it over standard input/output.

> TL;DR — build the sidecar, point your MCP client at the binary, make sure the
> GUI app has run at least once, then ask your LLM things like *"What models has
> Ollama served and what's the p95 latency?"*

## How it fits together

```
GUI app (writer) ──captures Ollama traffic──▶  telemetry.db  (SQLite, WAL)
                                                     ▲
                                                     │ read-only
                          ollama-telemetry-mcp ──────┘
                            (stdio sidecar)
                                  ▲
                                  │ stdio (MCP)
                        Claude Desktop / Claude Code
```

The GUI is the single writer; the sidecar is a separate read-only process. They
agree on the database location through one shared resolver. Full design:
[`ARCHITECTURE.md`](ARCHITECTURE.md).

## Prerequisites

1. **Go 1.26+** (to build the sidecar).
2. **The GUI app must have run at least once.** The sidecar only *reads* the
   database — it never creates it. If the GUI has never run, the sidecar exits
   with: `telemetry database not found — start the ollama-telemetry GUI app at
   least once before running the MCP sidecar`.
3. **Captured data.** For the tools to return anything useful, the GUI must have
   observed real Ollama traffic. Per-request capture uses WinDivert and needs
   the GUI to run **elevated (Administrator)**; otherwise it degrades to passive
   polling with no per-request detail. See [`passive-telemetry.md`](passive-telemetry.md).

## 1. Build the sidecar

From the repository root:

```sh
go build -o ollama-telemetry-mcp.exe ./cmd/ollama-telemetry-mcp
```

This produces `ollama-telemetry-mcp.exe`. Note its **absolute path** — MCP
clients launch it by full path. The binary takes **no flags**: it resolves the
database location itself (`%LOCALAPPDATA%\ollama-telemetry\telemetry.db` on
Windows).

## 2. Register it with an MCP client

### Claude Desktop

Edit `claude_desktop_config.json`
(`%APPDATA%\Claude\claude_desktop_config.json` on Windows) and add:

```json
{
  "mcpServers": {
    "ollama-telemetry": {
      "command": "C:\\absolute\\path\\to\\ollama-telemetry-mcp.exe"
    }
  }
}
```

Restart Claude Desktop. The `ollama-telemetry` tools appear in the tools menu.

### Claude Code

```sh
claude mcp add ollama-telemetry -- C:\absolute\path\to\ollama-telemetry-mcp.exe
```

Or commit a project-scoped `.mcp.json`:

```json
{
  "mcpServers": {
    "ollama-telemetry": {
      "command": "C:\\absolute\\path\\to\\ollama-telemetry-mcp.exe"
    }
  }
}
```

### Verify the connection (no client needed)

You can sanity-check that the binary starts and finds the DB by running it
directly — it will block waiting for an MCP client on stdio, which is correct.
If it instead prints the "database not found" error and exits, run the GUI app
first.

## 3. What it exposes

### Tools

| Tool | Parameters | Returns |
|------|------------|---------|
| `query_inferences` | `model`, `endpoint`, `status` (`in_progress` \| `completed` \| `metadata_only`), `since` (RFC3339), `until` (RFC3339), `limit` (int) — all optional | Matching inferences, most-recent-first. `limit: 0` means no cap. |
| `get_inference` | `id` (required) | `{ found, inference }` — the full record including request/response bodies and headers. Unknown id returns `found: false` (not an error). |
| `get_stats` | `model`, `since`, `until` — all optional | Aggregates over the filtered set: count, tokens/sec p50/p95, latency p50/p95 (ms), and per-model counts. |
| `list_models` | none | Distinct model names observed in stored inferences. |

All filters are optional and combine with AND. Empty/zero means "no constraint".

### Resources

| Resource URI | Content |
|--------------|---------|
| `inference://recent` | The 20 most recent inferences as a JSON array. |
| `inference://{id}` | A single full inference record as JSON. Unknown id → resource-not-found. |

### What's in an inference record

Each record carries: `id`, `at`, `endpoint`, `method`, `model`, `promptSize`,
`streaming`, `status`, `statusCode`, the captured `requestBody` /
`responseBody` (with `*Truncated` flags), `requestHeaders` / `responseHeaders`,
and `tokens` (prompt/eval counts, durations, derived tokens/sec and latency).
`tokens` is `null` when metrics are not applicable (in-progress or
metadata-only requests) — it is never faked as zero.

## 4. Try it

Once registered, prompt your LLM naturally:

- "List the Ollama models I've used recently."
- "What's the p95 tokens/sec for `llama3` in the last hour?"
- "Show me the most recent failed (`status` non-2xx) request and its body."
- "Compare average latency across models."

The LLM will call `list_models`, `get_stats`, `query_inferences`, and
`get_inference` as needed.

## Troubleshooting

| Symptom | Cause / fix |
|---------|-------------|
| `mcpServers: Invalid input: expected record, received undefined` (failed to parse) | Your config is missing the top-level `"mcpServers"` wrapper. The server entry must be nested **inside** `"mcpServers"`, not at the root of the file. |
| `telemetry database not found ...` | The GUI app has never run. Start it once so it creates and populates the DB. |
| Tools return empty results | No captured traffic yet. Run the GUI **elevated** and make some Ollama requests; per-request capture needs WinDivert (Administrator). |
| Client shows the server but no tools | Restart the client after editing its config; confirm the `command` path is absolute and correct. |
| Old data only / data "disappears" | Expected. Retention is **session-scoped** (a rolling cap of the 5000 most-recent inferences), mirroring Chrome DevTools' Network tab — there is no long-term archive. See [`ARCHITECTURE.md` → Retention](ARCHITECTURE.md#retention-session-scoped-rolling-count-cap-not-a-history-archive). |

## Notes

- The sidecar is **read-only** by design — it can never modify or corrupt the
  GUI's database (enforced at the SQLite connection level with
  `query_only(true)`).
- Only **stdio** transport is implemented today; an HTTP transport is a
  reserved, not-yet-built slot. See
  [`ARCHITECTURE.md` → Transport-decoupled MCP core](ARCHITECTURE.md#transport-decoupled-mcp-core).
- There are **no write/mutation tools** — the server only reads telemetry.
