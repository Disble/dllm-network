# Passive telemetry limits

Passive mode gives honest local observability. It does not give exact request analytics.

## Quick path

1. Treat **confirmed telemetry** as directly observed passive source data.
2. Treat **inferred activity** as a claim derived from passive signals.
3. Read the confidence and evidence labels before trusting an inferred conclusion.

## Confirmed vs inferred

| Topic | Meaning | Source | Required labels |
|-------|---------|--------|-----------------|
| Confirmed telemetry | Data the app observed directly | Ollama API polling, process metrics, owned loopback connections, host metrics | Timestamp / status |
| Inferred activity | A best-effort interpretation of passive signals | Activity engine rules over passive inputs | Confidence + Evidence |

## Exact fields unavailable in passive mode

- Exact request latency
- Exact token counts
- Exact HTTP status codes
- Exact request or response payloads
- Exact streaming chunks

## Captured payload retention

When the WinDivert capture source is active, exact request/response payloads
**are** observed and surfaced in the inference detail inspector. These bodies are
retained verbatim up to a safety ceiling.

- The ceiling is `inference.MaxBodyBytes` (`internal/telemetry/inference/extractor.go`),
  set to **16 MiB**. It exists to protect the **frontend render** — the detail
  view runs `JSON.parse` + pretty-print on the webview main thread, so a
  pathological multi-hundred-MiB body could freeze the UI. It is **not** a
  data-minimization cap.
- Backend memory stays bounded regardless: `store.Recent` keeps only the last 12
  completed inferences, so worst-case retention is ~12 × (request + response).
- Bodies above the ceiling are truncated and flagged via the `*Truncated` fields;
  the UI shows "Truncated at capture limit." Truncation cuts at an arbitrary byte,
  so a truncated body is no longer valid to pretty-print — expect raw view only.
- **If a legitimate Ollama response is being truncated, raising `MaxBodyBytes` is
  safe**: memory scales linearly and stays bounded by the 12-event retention.
  Re-check the frontend render cost before going far past ~16 MiB.

## Confidence and evidence rules

- **Confidence** expresses how strong the passive signals are for the inferred claim.
- **Evidence** lists the concrete passive observations that produced the inference.
- Confirmed fields stay separate from inferred fields in the UI and contracts.

## Reviewer checklist

- [ ] Confirmed telemetry is presented separately from inferred activity.
- [ ] Inferred claims include confidence.
- [ ] Inferred claims include evidence.
- [ ] Exact unavailable fields are never presented as confirmed metrics.
