package activity

import (
	"strings"
	"testing"
	"time"

	"dllm-network/internal/telemetry/ollama"
	"dllm-network/internal/telemetry/orchestrator"
	"dllm-network/internal/telemetry/system"
)

func TestEngineInferEmitsExplicitInferredActivity(t *testing.T) {
	t.Parallel()

	observedAt := time.Date(2026, time.June, 14, 22, 0, 0, 0, time.UTC)
	tests := []struct {
		name               string
		setup              func(*Engine)
		input              Input
		wantKind           Kind
		wantModel          string
		wantConfidence     Confidence
		wantEvidenceKinds  []EvidenceKind
		wantTruth          Truth
		wantObservedAt     time.Time
		wantRequestMention bool
	}{
		{
			name:           "confirmed running model with active loopback becomes inferred model loaded",
			input:          activityInput(observedAt, "gemma3", true, 4242, 2, system.StatusConfirmed),
			wantKind:       KindInferredModelLoaded,
			wantModel:      "gemma3",
			wantConfidence: ConfidenceHigh,
			wantEvidenceKinds: []EvidenceKind{
				EvidenceConfirmedRunningModel,
				EvidenceConfirmedProcessAvailable,
				EvidenceConfirmedConnectionActivityPresent,
			},
			wantTruth:      TruthInferred,
			wantObservedAt: observedAt,
		},
		{
			name: "confirmed running model change is inferred with explicit changed label",
			setup: func(engine *Engine) {
				engine.Infer(activityInput(observedAt.Add(-time.Minute), "llama3", true, 4242, 1, system.StatusConfirmed))
			},
			input:          activityInput(observedAt, "mistral", true, 4242, 1, system.StatusConfirmed),
			wantKind:       KindInferredModelChanged,
			wantModel:      "mistral",
			wantConfidence: ConfidenceHigh,
			wantEvidenceKinds: []EvidenceKind{
				EvidenceConfirmedRunningModel,
				EvidenceConfirmedProcessAvailable,
				EvidenceConfirmedConnectionActivityPresent,
			},
			wantTruth:      TruthInferred,
			wantObservedAt: observedAt,
		},
		{
			name:           "confirmed model with no loopback activity becomes inferred idle",
			input:          activityInput(observedAt, "gemma3", true, 4242, 0, system.StatusConfirmed),
			wantKind:       KindInferredIdle,
			wantModel:      "gemma3",
			wantConfidence: ConfidenceMedium,
			wantEvidenceKinds: []EvidenceKind{
				EvidenceConfirmedRunningModel,
				EvidenceConfirmedProcessAvailable,
				EvidenceConfirmedConnectionActivityAbsent,
			},
			wantTruth:      TruthInferred,
			wantObservedAt: observedAt,
		},
		{
			name:           "unavailable process metadata keeps activity unknown and low confidence",
			input:          activityInput(observedAt, "", false, 0, 0, system.StatusUnavailable),
			wantKind:       KindInferredUnknown,
			wantModel:      "",
			wantConfidence: ConfidenceLow,
			wantEvidenceKinds: []EvidenceKind{
				EvidenceSystemProcessUnavailable,
			},
			wantTruth:      TruthInferred,
			wantObservedAt: observedAt,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine()
			if tt.setup != nil {
				tt.setup(engine)
			}

			event := engine.Infer(tt.input)
			assertInferredActivity(t, event, expectedInferredActivity{
				Kind:       tt.wantKind,
				Model:      tt.wantModel,
				Confidence: tt.wantConfidence,
				Truth:      tt.wantTruth,
				ObservedAt: tt.wantObservedAt,
				Evidence:   tt.wantEvidenceKinds,
			})
		})
	}
}

func assertEvidenceKinds(t *testing.T, evidence []Evidence, want []EvidenceKind) {
	t.Helper()

	if len(evidence) != len(want) {
		t.Fatalf("expected %d evidence items, got %d: %+v", len(want), len(evidence), evidence)
	}

	for index, evidenceItem := range evidence {
		if evidenceItem.Kind != want[index] {
			t.Fatalf("expected evidence[%d] kind %q, got %q", index, want[index], evidenceItem.Kind)
		}
	}
}

func assertNoFalseConfirmedRequestClaims(t *testing.T, event Event) {
	t.Helper()

	if !strings.HasPrefix(string(event.Kind), "inferred-") {
		t.Fatalf("expected inferred event kind, got %q", event.Kind)
	}

	for _, evidence := range event.Evidence {
		if strings.Contains(string(evidence.Kind), "request") {
			t.Fatalf("expected passive evidence only, got %q", evidence.Kind)
		}
		if strings.Contains(strings.ToLower(evidence.Detail), "request") {
			t.Fatalf("expected evidence detail to avoid confirmed request claims, got %q", evidence.Detail)
		}
	}
}

// expectedInferredActivity holds the expected fields for assertInferredActivity,
// keeping the parameter count within SonarQube's S107 limit (≤7).
type expectedInferredActivity struct {
	Kind       Kind
	Model      string
	Confidence Confidence
	Truth      Truth
	ObservedAt time.Time
	Evidence   []EvidenceKind
}

// assertInferredActivity verifies an inferred event matches the expected
// expectedInferredActivity, and carries no false confirmed-request claims.
func assertInferredActivity(t *testing.T, event Event, want expectedInferredActivity) {
	t.Helper()
	if event.Kind != want.Kind {
		t.Fatalf("expected kind %q, got %q", want.Kind, event.Kind)
	}
	if event.Model != want.Model {
		t.Fatalf("expected model %q, got %q", want.Model, event.Model)
	}
	if event.Confidence != want.Confidence {
		t.Fatalf("expected confidence %q, got %q", want.Confidence, event.Confidence)
	}
	if event.Truth != want.Truth {
		t.Fatalf("expected truth %q, got %q", want.Truth, event.Truth)
	}
	if !event.ObservedAt.Equal(want.ObservedAt) {
		t.Fatalf("expected observed_at %s, got %s", want.ObservedAt, event.ObservedAt)
	}
	assertEvidenceKinds(t, event.Evidence, want.Evidence)
	assertNoFalseConfirmedRequestClaims(t, event)
}

func activityInput(observedAt time.Time, model string, processFound bool, pid int32, connectionCount int, processStatus system.SnapshotStatus) Input {
	connections := make([]system.ConnectionSample, 0, connectionCount)
	for index := 0; index < connectionCount; index++ {
		connections = append(connections, system.ConnectionSample{
			PID:           pid,
			LocalAddress:  "127.0.0.1",
			LocalPort:     11434,
			RemoteAddress: "127.0.0.1",
			RemotePort:    uint32(55000 + index),
			State:         "ESTABLISHED",
		})
	}

	runningModels := []ollama.RunningModel{}
	if model != "" {
		runningModels = append(runningModels, ollama.RunningModel{Name: model, Model: model})
	}

	processMeta := system.SnapshotMeta{
		Source:     system.SourceProcess,
		Provider:   system.DefaultProcessProvider,
		ObservedAt: observedAt,
		Status:     processStatus,
		Healthy:    processStatus == system.StatusConfirmed,
		Reachable:  processStatus == system.StatusConfirmed,
		Supported:  processStatus != system.StatusUnsupported,
	}
	if processStatus != system.StatusConfirmed {
		processMeta.Error = "process snapshot unavailable"
	}

	return Input{
		Ollama: ollama.PollSnapshot{
			Meta: ollama.SnapshotMeta{
				Source:     ollama.SourceHTTPAPI,
				Endpoint:   "/api/ps",
				ObservedAt: observedAt,
				Status:     ollama.StatusConfirmed,
				Reachable:  true,
			},
			Running: ollama.RunningModelsSnapshot{
				Meta: ollama.SnapshotMeta{
					Source:     ollama.SourceHTTPAPI,
					Endpoint:   "/api/ps",
					ObservedAt: observedAt,
					Status:     ollama.StatusConfirmed,
					Reachable:  true,
				},
				Models: runningModels,
			},
		},
		System: orchestrator.SystemSnapshot{
			CollectedAt: observedAt,
			Process: system.ProcessSnapshot{
				Meta:  processMeta,
				Found: processFound,
				Process: system.ProcessSample{
					PID:  pid,
					Name: "ollama.exe",
				},
			},
			Connections: system.ConnectionsSnapshot{
				Meta: system.SnapshotMeta{
					Source:     system.SourceConnections,
					Provider:   system.DefaultConnectionProvider,
					ObservedAt: observedAt,
					Status:     system.StatusConfirmed,
					Healthy:    true,
					Reachable:  true,
					Supported:  true,
				},
				PID:         pid,
				Connections: connections,
			},
		},
	}
}
