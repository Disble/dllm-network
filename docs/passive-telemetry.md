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

## Confidence and evidence rules

- **Confidence** expresses how strong the passive signals are for the inferred claim.
- **Evidence** lists the concrete passive observations that produced the inference.
- Confirmed fields stay separate from inferred fields in the UI and contracts.

## Reviewer checklist

- [ ] Confirmed telemetry is presented separately from inferred activity.
- [ ] Inferred claims include confidence.
- [ ] Inferred claims include evidence.
- [ ] Exact unavailable fields are never presented as confirmed metrics.
