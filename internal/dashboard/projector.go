package dashboard

import (
	"time"

	"ollama-telemetry/internal/activity"
	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/ollama"
	"ollama-telemetry/internal/telemetry/orchestrator"
	"ollama-telemetry/internal/telemetry/system"
)

type ProjectionInput struct {
	Ollama   ollama.PollSnapshot
	System   orchestrator.SystemSnapshot
	Activity activity.Event
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
}

type ConfirmedState struct {
	Ollama ConfirmedOllamaState `json:"ollama"`
	System ConfirmedSystemState `json:"system"`
}

type ConfirmedOllamaState struct {
	Status            string    `json:"status"`
	Reachable         bool      `json:"reachable"`
	Version           string    `json:"version,omitempty"`
	PrimaryModel      string    `json:"primaryModel,omitempty"`
	RunningModels     []string  `json:"runningModels"`
	CatalogModelCount int       `json:"catalogModelCount"`
	ObservedAt        time.Time `json:"observedAt"`
	LastConfirmedAt   time.Time `json:"lastConfirmedAt"`
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
	Mode                         string   `json:"mode"`
	ExactRequestLatencyAvailable bool     `json:"exactRequestLatencyAvailable"`
	ExactTokenCountsAvailable    bool     `json:"exactTokenCountsAvailable"`
	ExactPayloadAvailable        bool     `json:"exactPayloadAvailable"`
	ExactStatusAvailable         bool     `json:"exactStatusAvailable"`
	Notes                        []string `json:"notes"`
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
			Current: currentActivity,
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
		Passive: passiveMode(),
	}
}

func confirmedOllama(snapshot ollama.PollSnapshot) ConfirmedOllamaState {
	runningModels := make([]string, 0, len(snapshot.Running.Models))
	for _, model := range snapshot.Running.Models {
		if model.Name != "" {
			runningModels = append(runningModels, model.Name)
			continue
		}

		runningModels = append(runningModels, model.Model)
	}

	primaryModel := ""
	if len(runningModels) > 0 {
		primaryModel = runningModels[0]
	}

	return ConfirmedOllamaState{
		Status:            string(snapshot.Meta.Status),
		Reachable:         snapshot.Meta.Reachable,
		Version:           snapshot.Version.Version,
		PrimaryModel:      primaryModel,
		RunningModels:     runningModels,
		CatalogModelCount: len(snapshot.Catalog.Models),
		ObservedAt:        firstNonZero(snapshot.Running.Meta.ObservedAt, snapshot.Meta.ObservedAt),
		LastConfirmedAt:   firstNonZero(snapshot.Running.Meta.LastConfirmedAt, snapshot.Meta.LastConfirmedAt),
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
		return nil, nil
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
	return confirmedModels, append([]activity.Event(nil), activities...)
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

func passiveMode() PassiveLimitMode {
	return PassiveLimitMode{
		Mode:                         "passive-only",
		ExactRequestLatencyAvailable: false,
		ExactTokenCountsAvailable:    false,
		ExactPayloadAvailable:        false,
		ExactStatusAvailable:         false,
		Notes: []string{
			"Exact request latency is unavailable in passive mode.",
			"Exact token counts are unavailable in passive mode.",
			"Exact request and response payloads are unavailable in passive mode.",
			"Exact HTTP status results are unavailable in passive mode.",
		},
	}
}

func firstNonZero(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}

	return time.Time{}
}
