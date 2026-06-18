// Package dashboard projects raw telemetry poll snapshots into the frontend
// Snapshot contract. The Projector is the sole anti-corruption boundary between
// domain types (ollama, system, inference) and the wire-stable Snapshot.
//
// RunningModels widening decision (WU4, 2026-06-16): ADDITIVE.
// The legacy RunningModels []string field is kept for back-compat with existing
// frontend consumers. The new RunningModelDetails []RunningModelView field
// carries size, size_vram, parameter_size, quantization_level, context_length,
// and expires_at — already decoded in ollama/types.go RunningModel. WU8
// frontend work consumes RunningModelDetails; WU4-era consumers continue to use
// RunningModels unmodified.
package dashboard

import (
	"time"

	"ollama-telemetry/internal/activity"
	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
	"ollama-telemetry/internal/telemetry/ollama"
	"ollama-telemetry/internal/telemetry/orchestrator"
	"ollama-telemetry/internal/telemetry/system"
)

// CaptureInput carries per-category signals that captureMode() uses to decide
// which PassiveLimitMode flags to enable. The zero value represents a disabled
// or unelevated capture source (all flags false).
type CaptureInput struct {
	// SourceActive is true when the WinDivert (or fake) capture source is
	// running and elevated. When false the remaining fields are ignored and
	// all five flags remain false.
	SourceActive bool

	// Per-category data-presence signals — independent of each other so that
	// a /api/tags exchange (HasTokenCounts=false) does not suppress the flags
	// for categories that are genuinely available.
	HasLatency      bool // at least one completed exchange with latency data
	HasTokenCounts  bool // at least one exchange with eval_count / prompt_eval_count
	HasPayload      bool // request/response body decoded
	HasStatus       bool // HTTP status code captured
	HasStreamChunks bool // streamed NDJSON chunks decoded

	// UnelevatedNote is included verbatim in PassiveLimitMode.Notes when
	// SourceActive is false and the note is non-empty.
	UnelevatedNote string
}

// InferenceState holds the live and recent inference activity produced by the
// capture pipeline. It crosses the anti-corruption boundary from
// internal/telemetry/inference into the Snapshot contract.
type InferenceState struct {
	// Current is the most recent in-progress or just-completed Inference. It
	// is zero when no capture data has been observed yet.
	Current inference.Inference `json:"current"`

	// Recent holds the last N completed Inference events in chronological
	// order. It is empty when capture is disabled or no inferences have
	// completed yet.
	Recent []inference.Inference `json:"recent"`
}

// ProjectionInput carries all data sources needed to build one Snapshot.
type ProjectionInput struct {
	Ollama   ollama.PollSnapshot
	System   orchestrator.SystemSnapshot
	Activity activity.Event

	// Capture provides per-category data-presence signals derived from the
	// WinDivert capture pipeline. The zero value means capture is disabled.
	Capture CaptureInput

	// Inference carries the live and recent inference activity produced by
	// the capture pipeline. The zero value means no inference data yet.
	Inference InferenceState
}

type RecentReader interface {
	Snapshots() []store.Snapshot
	Activities() []activity.Event
}

type Snapshot struct {
	PublishedAt time.Time        `json:"publishedAt"`
	Confirmed   ConfirmedState   `json:"confirmed"`
	Inferred    InferredState    `json:"inferred"`
	Recent      RecentState      `json:"recent"`
	Health      CollectorHealth  `json:"health"`
	Passive     PassiveLimitMode `json:"passive"`
	// Inference carries live and recent inference activity from the capture
	// pipeline. Zero value when capture is disabled or no inference yet seen.
	Inference InferenceState `json:"inference"`
}

type ConfirmedState struct {
	Ollama ConfirmedOllamaState `json:"ollama"`
	System ConfirmedSystemState `json:"system"`
}

// RunningModelView is the enriched per-model view surfaced in
// ConfirmedOllamaState.RunningModelDetails. Fields mirror the ollama API /api/ps
// response, already decoded in ollama/types.go RunningModel.
type RunningModelView struct {
	Name              string    `json:"name"`
	Size              int64     `json:"size"`
	SizeVRAM          int64     `json:"sizeVram"`
	ParameterSize     string    `json:"parameterSize"`
	QuantizationLevel string    `json:"quantizationLevel"`
	ContextLength     int       `json:"contextLength"`
	ExpiresAt         time.Time `json:"expiresAt"`
}

type ConfirmedOllamaState struct {
	Status              string             `json:"status"`
	Reachable           bool               `json:"reachable"`
	Version             string             `json:"version,omitempty"`
	PrimaryModel        string             `json:"primaryModel,omitempty"`
	RunningModels       []string           `json:"runningModels"`
	RunningModelDetails []RunningModelView `json:"runningModelDetails"`
	CatalogModelCount   int                `json:"catalogModelCount"`
	ObservedAt          time.Time          `json:"observedAt"`
	LastConfirmedAt     time.Time          `json:"lastConfirmedAt"`
}

type ConfirmedSystemState struct {
	ObservedAt  time.Time                `json:"observedAt"`
	Process     ConfirmedProcessState    `json:"process"`
	Connections ConfirmedConnectionState `json:"connections"`
	Host        ConfirmedHostState       `json:"host"`
}

type ConfirmedProcessState struct {
	Status     string  `json:"status"`
	Found      bool    `json:"found"`
	PID        int32   `json:"pid"`
	CPUPercent float64 `json:"cpuPercent"`
	RSSBytes   uint64  `json:"rssBytes"`
}

type ConfirmedConnectionState struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type ConfirmedHostState struct {
	Status           string  `json:"status"`
	CPUPercent       float64 `json:"cpuPercent"`
	MemoryUsedBytes  uint64  `json:"memoryUsedBytes"`
	MemoryTotalBytes uint64  `json:"memoryTotalBytes"`
}

type InferredState struct {
	Current activity.Event   `json:"current"`
	Recent  []activity.Event `json:"recent"`
}

type RecentState struct {
	ConfirmedModels []RecentConfirmedModel `json:"confirmedModels"`
}

type RecentConfirmedModel struct {
	ObservedAt time.Time `json:"observedAt"`
	Model      string    `json:"model"`
}

type CollectorHealth struct {
	Ollama      HealthState `json:"ollama"`
	Process     HealthState `json:"process"`
	Connections HealthState `json:"connections"`
	Host        HealthState `json:"host"`
}

type HealthState struct {
	Status     string    `json:"status"`
	Healthy    bool      `json:"healthy"`
	Supported  bool      `json:"supported"`
	ObservedAt time.Time `json:"observedAt"`
	Error      string    `json:"error,omitempty"`
}

type PassiveLimitMode struct {
	Mode                          string   `json:"mode"`
	ExactRequestLatencyAvailable  bool     `json:"exactRequestLatencyAvailable"`
	ExactTokenCountsAvailable     bool     `json:"exactTokenCountsAvailable"`
	ExactPayloadAvailable         bool     `json:"exactPayloadAvailable"`
	ExactStatusAvailable          bool     `json:"exactStatusAvailable"`
	ExactStreamingChunksAvailable bool     `json:"exactStreamingChunksAvailable"`
	Notes                         []string `json:"notes"`
}

type Projector struct {
	now func() time.Time
}

func NewProjector(now func() time.Time) *Projector {
	if now == nil {
		now = time.Now
	}

	return &Projector{now: now}
}

func (projector *Projector) Project(input ProjectionInput, recent RecentReader) Snapshot {
	recentSnapshots, recentActivities := recentState(recent)
	currentActivity := input.Activity
	if currentActivity.Kind == "" && len(recentActivities) > 0 {
		currentActivity = recentActivities[len(recentActivities)-1]
	}

	return Snapshot{
		PublishedAt: projector.now(),
		Confirmed: ConfirmedState{
			Ollama: confirmedOllama(input.Ollama),
			System: confirmedSystem(input.System),
		},
		Inferred: InferredState{
			Current: normalizeActivityEvent(currentActivity),
			Recent:  recentActivities,
		},
		Recent: RecentState{
			ConfirmedModels: recentSnapshots,
		},
		Health: CollectorHealth{
			Ollama:      ollamaHealth(input.Ollama.Meta),
			Process:     systemHealth(input.System.Process.Meta),
			Connections: systemHealth(input.System.Connections.Meta),
			Host:        systemHealth(input.System.Host.Meta),
		},
		Passive:   normalizePassiveLimitMode(captureMode(input.Capture)),
		Inference: normalizeInferenceState(input.Inference),
	}
}

func normalizeActivityEvent(event activity.Event) activity.Event {
	if event.Evidence == nil {
		event.Evidence = []activity.Evidence{}
	}
	return event
}

func normalizeInferenceState(state InferenceState) InferenceState {
	if state.Recent == nil {
		state.Recent = []inference.Inference{}
	}
	return state
}

func normalizePassiveLimitMode(mode PassiveLimitMode) PassiveLimitMode {
	if mode.Notes == nil {
		mode.Notes = []string{}
	}
	return mode
}

func confirmedOllama(snapshot ollama.PollSnapshot) ConfirmedOllamaState {
	runningModels := make([]string, 0, len(snapshot.Running.Models))
	runningModelDetails := make([]RunningModelView, 0, len(snapshot.Running.Models))

	for _, model := range snapshot.Running.Models {
		// Legacy string slice: prefer Name (full tag), fall back to Model field.
		name := model.Name
		if name == "" {
			name = model.Model
		}
		runningModels = append(runningModels, name)

		// Enriched view: surface all fields already decoded in ollama/types.go.
		runningModelDetails = append(runningModelDetails, RunningModelView{
			Name:              name,
			Size:              model.Size,
			SizeVRAM:          model.SizeVRAM,
			ParameterSize:     model.Details.ParameterSize,
			QuantizationLevel: model.Details.QuantizationLevel,
			ContextLength:     model.ContextLength,
			ExpiresAt:         model.ExpiresAt,
		})
	}

	primaryModel := ""
	if len(runningModels) > 0 {
		primaryModel = runningModels[0]
	}

	return ConfirmedOllamaState{
		Status:              string(snapshot.Meta.Status),
		Reachable:           snapshot.Meta.Reachable,
		Version:             snapshot.Version.Version,
		PrimaryModel:        primaryModel,
		RunningModels:       runningModels,
		RunningModelDetails: runningModelDetails,
		CatalogModelCount:   len(snapshot.Catalog.Models),
		ObservedAt:          firstNonZero(snapshot.Running.Meta.ObservedAt, snapshot.Meta.ObservedAt),
		LastConfirmedAt:     firstNonZero(snapshot.Running.Meta.LastConfirmedAt, snapshot.Meta.LastConfirmedAt),
	}
}

func confirmedSystem(snapshot orchestrator.SystemSnapshot) ConfirmedSystemState {
	return ConfirmedSystemState{
		ObservedAt: snapshot.CollectedAt,
		Process: ConfirmedProcessState{
			Status:     string(snapshot.Process.Meta.Status),
			Found:      snapshot.Process.Found,
			PID:        snapshot.Process.Process.PID,
			CPUPercent: snapshot.Process.Process.CPUPercent,
			RSSBytes:   snapshot.Process.Process.RSSBytes,
		},
		Connections: ConfirmedConnectionState{
			Status: string(snapshot.Connections.Meta.Status),
			Count:  len(snapshot.Connections.Connections),
		},
		Host: ConfirmedHostState{
			Status:           string(snapshot.Host.Meta.Status),
			CPUPercent:       snapshot.Host.Metrics.CPUPercent,
			MemoryUsedBytes:  snapshot.Host.Metrics.MemoryUsedBytes,
			MemoryTotalBytes: snapshot.Host.Metrics.MemoryTotalBytes,
		},
	}
}

func recentState(recent RecentReader) ([]RecentConfirmedModel, []activity.Event) {
	if recent == nil {
		return []RecentConfirmedModel{}, []activity.Event{}
	}

	snapshots := recent.Snapshots()
	confirmedModels := make([]RecentConfirmedModel, 0, len(snapshots))
	for _, snapshot := range snapshots {
		confirmedModels = append(confirmedModels, RecentConfirmedModel{
			ObservedAt: snapshot.ObservedAt,
			Model:      snapshot.ConfirmedModel,
		})
	}

	activities := recent.Activities()
	recentActivities := make([]activity.Event, 0, len(activities))
	for _, event := range activities {
		recentActivities = append(recentActivities, normalizeActivityEvent(event))
	}
	return confirmedModels, recentActivities
}

func ollamaHealth(meta ollama.SnapshotMeta) HealthState {
	return HealthState{
		Status:     string(meta.Status),
		Healthy:    meta.Reachable && meta.Status == ollama.StatusConfirmed,
		Supported:  true,
		ObservedAt: meta.ObservedAt,
		Error:      meta.Error,
	}
}

func systemHealth(meta system.SnapshotMeta) HealthState {
	return HealthState{
		Status:     string(meta.Status),
		Healthy:    meta.Healthy,
		Supported:  meta.Supported,
		ObservedAt: meta.ObservedAt,
		Error:      meta.Error,
	}
}

// captureMode computes the PassiveLimitMode from the per-category data signals
// produced by the WinDivert capture pipeline. Each flag is set independently
// based on whether that category of data was actually observed — the function
// never blanket-flips all flags at once.
//
// When SourceActive is false (capture disabled, unelevated, or not yet started),
// all five flags remain false and an explanatory note is included. When
// SourceActive is true, each flag is set only when the corresponding data
// category was present in the observed exchange.
func captureMode(input CaptureInput) PassiveLimitMode {
	if !input.SourceActive {
		notes := []string{
			"Exact request latency is unavailable in passive mode.",
			"Exact token counts are unavailable in passive mode.",
			"Exact request and response payloads are unavailable in passive mode.",
			"Exact HTTP status results are unavailable in passive mode.",
			"Exact streaming chunks are unavailable in passive mode.",
		}
		if input.UnelevatedNote != "" {
			notes = append(notes, input.UnelevatedNote)
		}
		return PassiveLimitMode{
			Mode:  "passive-only",
			Notes: notes,
		}
	}

	// Source is active: set flags per-category and collect honest notes for
	// any category that is still unavailable in this exchange.
	plm := PassiveLimitMode{
		Mode:                          "capture-active",
		ExactRequestLatencyAvailable:  input.HasLatency,
		ExactTokenCountsAvailable:     input.HasTokenCounts,
		ExactPayloadAvailable:         input.HasPayload,
		ExactStatusAvailable:          input.HasStatus,
		ExactStreamingChunksAvailable: input.HasStreamChunks,
	}

	// Per-category honest notes for any unavailable category.
	if !input.HasLatency {
		plm.Notes = append(plm.Notes, "Exact request latency unavailable for this exchange (no timing data).")
	}
	if !input.HasTokenCounts {
		plm.Notes = append(plm.Notes, "Exact token counts unavailable for this exchange (no eval_count field).")
	}
	if !input.HasPayload {
		plm.Notes = append(plm.Notes, "Exact request/response payload unavailable for this exchange.")
	}
	if !input.HasStatus {
		plm.Notes = append(plm.Notes, "Exact HTTP status unavailable for this exchange.")
	}
	if !input.HasStreamChunks {
		plm.Notes = append(plm.Notes, "Streaming chunk data unavailable for this exchange (non-streaming endpoint).")
	}

	return plm
}

func firstNonZero(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}

	return time.Time{}
}
