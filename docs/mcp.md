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

The MCP surface is intentionally small:

- **Exactly 3 tools**.
- **Zero MCP resources**.
- **Stable inference ids** across all three steps.

### Quick path

1. Call `resolve_inference_context` to learn the searchable universe.
2. Call `search_inferences` with filters and a page size.
3. Pick one stable `id` from the summaries and call `get_inference_context` only for the sections or body chunk you actually need.

### Tools

| Tool | When to use | Key inputs | Key outputs |
|------|-------------|------------|-------------|
| `resolve_inference_context` | First call in a session, or when you need orientation | none | `models`, `endpoints`, `statuses`, `timeRange`, `counts`, `supportedFilters` |
| `search_inferences` | Find candidate requests without pulling heavy payloads | `model`, `endpoint`, `status`, `since`, `until`, `limit`, `cursor` | `items[]` lightweight summaries plus `nextCursor` |
| `get_inference_context` | Inspect one chosen inference in bounded detail | `id`, optional `sections[]`, optional `body{name,offset,limit}` | `availableSections`, requested sections, optional `bodyChunk` |

### `resolve_inference_context`

Use this to discover what exists before searching.

| Field | Meaning |
|-------|---------|
| `models[]` | Distinct model names with counts |
| `endpoints[]` | Distinct endpoints with counts |
| `statuses[]` | Distinct lifecycle statuses with counts |
| `timeRange.oldest/latest` | Bounds of stored telemetry |
| `counts.total` | Total stored inference rows |
| `supportedFilters` | Always declares `model`, `endpoint`, `status`, `since`, `until` |

This response never includes request bodies, response bodies, or per-event detail.

### `search_inferences`

Use this to page through lightweight candidates.

| Input | Notes |
|-------|-------|
| `limit` | Defaults to `20`, maximum `100` |
| `cursor` | Opaque token returned by the previous page |
| `model`, `endpoint`, `status`, `since`, `until` | Optional filters combined with AND |

Each summary contains stable fields only: `id`, `at`, `model`, `endpoint`, `method`, `status`, `statusCode`, `streaming`, and `promptSize`.

Ordering is deterministic: **`at DESC, id DESC`**. That secondary `id` tie-breaker matters when several rows share the same timestamp. Repeating the same search on an unchanged dataset yields the same order and the same page boundaries.

### `get_inference_context`

Use this only after you already know the target `id`.

#### Sections

Supported `sections[]` values:

- `metadata`
- `tokens`
- `request_headers`
- `response_headers`

If `sections` is omitted, the server returns `metadata` by default.

#### Body chunks

Bodies are read on demand through `body`:

```json
{
  "id": "inf_123",
  "sections": ["metadata", "tokens"],
  "body": {
    "name": "request_body",
    "offset": 0,
    "limit": 4096
  }
}
```

`bodyChunk` returns:

| Field | Meaning |
|-------|---------|
| `name` | `request_body` or `response_body` |
| `offset` / `limit` | Slice requested |
| `nextOffset` | Next byte position to request |
| `hasMore` | More bytes remain in the stored body |
| `totalBytes` | Stored body length |
| `content` | Returned body slice |
| `truncated` | The original capture was truncated before persistence |

Requesting an unavailable section or an exhausted body range is still a successful call. The server reports availability and returns an empty slice for that part instead of fabricating data.

### What is intentionally gone

The server no longer exposes these legacy entry points:

- `query_inferences`
- `get_inference`
- `get_stats`
- `list_models`
- `inference://recent`
- `inference://{id}`

## 4. Try it

Once registered, prompt your LLM naturally:

- "What models and endpoints are available in the stored telemetry?"
- "Search the last hour for failed `/api/generate` calls and show me the newest 5 summaries."
- "Open inference `...` and give me metadata, token stats, and the first 4 KiB of the response body."
- "Continue from this `nextCursor` and fetch the next page."

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
