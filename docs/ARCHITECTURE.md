# Architecture: MCP serving

This is a living document. The rules it describes are also machine-enforced
by `.golangci.yml` (depguard) — if this doc and the linter ever disagree,
the linter wins; fix the doc.

## Quick path

1. Telemetry flows one direction only: **capture → bus → SQLite (write)**,
   then separately **SQLite (read) → MCP**. Nothing reads back from MCP into
   capture.
2. SQLite is the single source of truth shared by the GUI (writer) and the
   stdio sidecar (reader). They are two different OS processes opening the
   same file.
3. Three boundaries are enforced by the linter, not just convention — see
   [Enforced boundaries](#enforced-boundaries). If you need to violate one,
   that is a design conversation, not a one-line edit.

## Data flow

```
capture (WinDivert)            read side
       |                            |
       v                            v
   events.Bus  ----publish---->  (nothing subscribes here for reads)
       |
       v
 persistence.Subscriber (non-blocking enqueue, batch flush)
       |
       v
   sqlite.Store (writer, WAL)  <-- same file -->  sqlite.Store (OpenReadOnly)
                                                         |
                                                         v
                                              store.InferenceReader
                                                 /              \
                                                v                v
                                       dashboard (GUI,    internal/mcp (tools +
                                       store.Recent ring)  resources) -> stdio
                                                                sidecar binary
```

| Stage | Package | Responsibility |
|-------|---------|-----------------|
| Capture | `internal/capture` | Observes Ollama traffic (WinDivert), produces `inference.Inference` |
| Write trigger | `internal/app/capture_pipeline.go` | On completion: updates the live ring (`store.Recent`) AND publishes to `events.Bus` |
| Async persistence | `internal/persistence` | Subscribes to the bus, non-blocking drop-oldest enqueue, batches into SQLite |
| Durable store | `internal/store/sqlite` | WAL-mode SQLite, single writer (GUI), read-only reader (sidecar) |
| Read port | `internal/store` (`InferenceReader`/`InferenceWriter`) | Segregated interfaces; only `sqlite.Store` implements both |
| MCP core | `internal/mcp` | Tools + resources over `store.InferenceReader`, transport-agnostic |
| Sidecar | `cmd/ollama-telemetry-mcp` | Standalone binary: opens the DB read-only, serves MCP over stdio |

## The WAL seam: one writer, many readers

SQLite's WAL mode allows exactly the split this project needs: one
read-write connection (the GUI app, started once per `ollama-telemetry`
session) and any number of read-only connections (the stdio sidecar,
launched independently by an MCP client such as Claude Desktop).

| Connection | DSN options | Opens via |
|------------|-------------|-----------|
| Writer (GUI) | `_pragma=busy_timeout(5000)`, `_pragma=journal_mode(WAL)`, `_txlock=immediate` | `sqlite.Open` |
| Reader (sidecar) | `mode=ro`, same `busy_timeout`/`WAL` pragmas, **`_pragma=query_only(true)`** | `sqlite.OpenReadOnly` |

**Why `query_only`, not just `mode=ro`:** `mode=ro` is a SQLite URI-query
parameter — it is only honored when the DSN is in `file:` URI form. This
project's DSN is a bare path with query params appended
(`path+"?mode=ro&..."`), which `modernc.org/sqlite` does not treat as a URI,
so `mode=ro` alone is silently ignored and the connection would otherwise
accept writes. `_pragma=query_only(true)` is honored regardless of DSN form
(pragmas apply via repeated `_pragma=name(value)` params either way) and
makes SQLite itself reject every write statement at the connection level.
`mode=ro` is kept anyway as a harmless hint. See
`internal/store/sqlite/readonly_test.go` (`TestOpenReadOnly_RejectsWrites`)
for the regression test that locks this in.

**What `query_only` does NOT do:** prevent file creation on a missing
database. That is a separate failure mode (a bare-path DSN against a
nonexistent file silently creates an empty one) and is handled one layer up,
at the sidecar's wiring boundary: `cmd/ollama-telemetry-mcp/run.go` checks
`os.Stat` on the resolved path *before* ever calling `OpenReadOnly`, and
fails fast with a clear "start the GUI app first" message if the file does
not exist yet.

Both connections resolve the same path via `sqlite.DefaultPath()`
(`%LOCALAPPDATA%/ollama-telemetry/telemetry.db` on Windows, via
`os.UserCacheDir()`) — sharing one resolver, not two independent ones,
guarantees the GUI and the sidecar can never disagree about where the
database lives.

## Retention: session-scoped rolling count cap, not a history archive

This app mirrors **Chrome DevTools' Network tab**: captured requests are a
working view of the current session, not a permanent audit log. Following
that reference model, SQLite retention is a **rolling COUNT cap** — keep the
`defaultRetentionCount` (5000) most-recent inferences, oldest rows pruned
first — with **no age-based cap**. This is a deliberate scope boundary, not
an oversight:

| What this is NOT | Why |
|-------------------|-----|
| A long-term history archive | The reference UX (DevTools) doesn't keep one either; old entries roll off |
| Age-based retention | A count cap alone already satisfies the spec's "must not grow unbounded" requirement; adding an age cap would be unused complexity for a session-scoped tool |
| Cleared on app restart | Explicitly **out of scope** for this slice — the rolling window currently persists across restarts (a possible future fidelity enhancement to fully match DevTools' clear-on-reload, not implemented) |

**Why SQLite at all, if it's not a history archive:** durable-on-disk
storage here is justified by **cross-process IPC**, not by long-term
retention. The GUI (writer) and the stdio sidecar (reader, `cmd/
ollama-telemetry-mcp`) are two separate OS processes — an in-memory store
cannot be shared between them, so a shared file is the seam, and SQLite/WAL
is the simplest pure-Go way to make that seam safe for one writer + many
readers (see [The WAL seam](#the-wal-seam-one-writer-many-readers) above).
Retention scope and the choice-of-storage-medium are two independent
decisions; conflating them was the root cause of an earlier gap (the
`Prune` method existed and was unit-tested but had no production caller —
see `internal/persistence/retention_test.go`'s
`TestSubscriber_PruneOnFlush_LiveWiring` for the regression test that locks
the fix in).

**Where it's wired:** `internal/persistence`'s batcher calls
`Writer.Prune(ctx, defaultRetentionCount, pruneAgeDisabled)` immediately
after every successful batch flush (`internal/persistence/batch.go`) — in
the same drain goroutine that performs `Save`, never in the bus-handler
goroutine. This keeps pruning flush-driven (no separate ticker/cron) while
preserving the non-blocking `bus.Publish` contract: a slow or failing prune
can only ever add latency to the next flush cycle, never to the capture
loop that publishes events.

## The ports: why two reader/writer interfaces, not one

```go
type InferenceWriter interface { Save(ctx, []inference.Inference) error }
type InferenceReader interface {
    Query(ctx, Filter) ([]inference.Inference, error)
    Get(ctx, id string) (inference.Inference, bool, error)
    Stats(ctx, Filter) (Stats, error)
    Models(ctx) ([]string, error)
}
```

`store.Recent` (the in-memory 12-item ring used by the live dashboard) does
**not** implement either interface, and never will.

| Concern | `store.Recent` (ring) | `sqlite.Store` (durable) |
|---------|------------------------|---------------------------|
| Lifetime | One GUI process, volatile | Cross-process, durable on disk |
| Size | Fixed at 12 most-recent | Bounded by rolling count cap (default 5000, see [Retention](#retention-session-scoped-rolling-count-cap-not-a-history-archive)) |
| Consumer | Live dashboard projection | MCP read side, future analytics |
| Latency budget | Must be free — hot path | Can tolerate disk I/O |

Coupling these through one interface would force the dashboard's hot path
(every completed inference) to pay for a disk write, or force the durable
store to support a ring's volatile semantics. Keeping them separate lets
each optimize for its own consumer. They share one trigger point
(`capture_pipeline.go`'s completion branch calls both
`recent.RecordInferenceCompletion` and `bus.Publish`) but never share code
or a type.

## Transport-decoupled MCP core

`internal/mcp` registers tools (`query_inferences`, `get_inference`,
`get_stats`, `list_models`) and resources (`inference://{id}`,
`inference://recent`) against an injected `store.InferenceReader` — it never
imports `sqlite` directly, only the port. The SDK's `mcp.Transport` is the
seam between the core and how bytes actually move:

```go
func RunStdio(ctx context.Context, srv *mcp.Server, transport mcp.Transport) error
func Serve(ctx context.Context, srv *mcp.Server) error // production: stdio only, for now
```

| Transport | Status | Why |
|-----------|--------|-----|
| stdio | Implemented (`Serve`) | What every MCP client (Claude Desktop, etc.) launches today |
| HTTP | Not implemented — reserved slot | Would add a sibling `ServeHTTP` calling `RunStdio`'s same pattern with a different `mcp.Transport`; touches zero registration code in `server.go`/`tools_*.go`/`resources.go` |

Tests exercise the core via `mcp.NewInMemoryTransports()` — no real stdio,
no real DB — proving the registration wiring independent of any transport.

## Enforced boundaries

These three rules are encoded in `.golangci.yml` as `depguard` rules. Run
`bun run lint:go` (or `golangci-lint run ./...`) to check them; do not rely
on code review alone.

| Rule | What's denied | From | Why |
|------|----------------|------|-----|
| `inference-domain-purity` | `database/sql` | `internal/telemetry/inference/**` | Domain type stays driver-free; persistence is `internal/store/sqlite`'s job |
| `inference-domain-purity` | `github.com/modelcontextprotocol/go-sdk` | `internal/telemetry/inference/**` | Domain type stays transport-free; MCP wiring is `internal/mcp`'s job |
| `mcp-not-capture` | `ollama-telemetry/internal/capture` | `internal/mcp/**` | MCP is read-only; it must depend on `store.InferenceReader`, never reach into the write-side capture pipeline |
| `sdk-confined-to-mcp` | `github.com/modelcontextprotocol/go-sdk` | everywhere except `internal/mcp/**` | Keeps every other package buildable/testable without the SDK; confines protocol churn to one package |

The linter enforces the **what** (the import is or isn't allowed); this doc
explains the **why** (the architectural reason each boundary exists). If a
new package legitimately needs to cross one of these lines, that is a
decision to update this doc and `.golangci.yml` together — not to add a
`//nolint`.

## Reviewer checklist

- [ ] New code in `internal/telemetry/inference` does not import a storage
      driver or the MCP SDK.
- [ ] New code in `internal/mcp` depends on `store.InferenceReader`, not on
      `internal/capture` or `sqlite` directly.
- [ ] New code outside `internal/mcp` does not import the MCP SDK.
- [ ] Any new SQLite connection goes through `sqlite.Open` (writer) or
      `sqlite.OpenReadOnly` (reader) — never a hand-rolled DSN.
- [ ] `bun run lint:go` is clean before merging a change that touches any of
      the packages above.
