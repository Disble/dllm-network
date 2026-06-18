package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"ollama-telemetry/internal/activity"
	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
	"ollama-telemetry/internal/telemetry/ollama"
	"ollama-telemetry/internal/telemetry/orchestrator"
	"ollama-telemetry/internal/telemetry/system"
)

func TestProjectorProjectsConfirmedAndInferredSnapshot(t *testing.T) {
	t.Parallel()

	publishedAt := time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC)
	recent := stubRecentReader{
		snapshots: []store.Snapshot{
			{ObservedAt: publishedAt.Add(-2 * time.Minute), ConfirmedModel: "llama3"},
			{ObservedAt: publishedAt.Add(-time.Minute), ConfirmedModel: "gemma3"},
		},
		activities: []activity.Event{
			{Kind: activity.KindInferredIdle, Truth: activity.TruthInferred, Confidence: activity.ConfidenceMedium, Model: "gemma3", ObservedAt: publishedAt.Add(-2 * time.Minute)},
			{Kind: activity.KindInferredModelLoaded, Truth: activity.TruthInferred, Confidence: activity.ConfidenceHigh, Model: "mistral", ObservedAt: publishedAt.Add(-time.Minute)},
		},
	}
	projector := NewProjector(func() time.Time { return publishedAt })

	snapshot := projector.Project(ProjectionInput{
		Ollama: ollama.PollSnapshot{
			Meta: ollama.SnapshotMeta{ObservedAt: publishedAt.Add(-5 * time.Second), Status: ollama.StatusConfirmed, Reachable: true},
			Version: ollama.VersionSnapshot{
				Meta:    ollama.SnapshotMeta{ObservedAt: publishedAt.Add(-5 * time.Second), Status: ollama.StatusConfirmed, Reachable: true},
				Version: "0.8.0",
			},
			Running: ollama.RunningModelsSnapshot{
				Meta:   ollama.SnapshotMeta{ObservedAt: publishedAt.Add(-5 * time.Second), Status: ollama.StatusConfirmed, Reachable: true},
				Models: []ollama.RunningModel{{Name: "mistral", Model: "mistral"}},
			},
			Catalog: ollama.CatalogSnapshot{
				Meta:   ollama.SnapshotMeta{ObservedAt: publishedAt.Add(-5 * time.Second), Status: ollama.StatusConfirmed, Reachable: true},
				Models: []ollama.CatalogModel{{Name: "mistral"}, {Name: "gemma3"}},
			},
		},
		System: orchestrator.SystemSnapshot{
			CollectedAt: publishedAt.Add(-3 * time.Second),
			Process: system.ProcessSnapshot{
				Meta:    system.SnapshotMeta{ObservedAt: publishedAt.Add(-3 * time.Second), Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true},
				Found:   true,
				Process: system.ProcessSample{PID: 4242, Name: "ollama.exe", CPUPercent: 13.5, RSSBytes: 1024},
			},
			Connections: system.ConnectionsSnapshot{
				Meta:        system.SnapshotMeta{ObservedAt: publishedAt.Add(-3 * time.Second), Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true},
				PID:         4242,
				Connections: []system.ConnectionSample{{PID: 4242, LocalAddress: "127.0.0.1", LocalPort: 11434, RemoteAddress: "127.0.0.1", RemotePort: 56111, State: "ESTABLISHED"}},
			},
			Host: system.HostSnapshot{
				Meta:    system.SnapshotMeta{ObservedAt: publishedAt.Add(-3 * time.Second), Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true},
				Metrics: system.HostMetrics{CPUPercent: 24.2, MemoryUsedBytes: 4096, MemoryTotalBytes: 8192},
			},
		},
		Activity: activity.Event{
			Kind:       activity.KindInferredModelChanged,
			Truth:      activity.TruthInferred,
			Model:      "mistral",
			Confidence: activity.ConfidenceHigh,
			Evidence:   []activity.Evidence{{Kind: activity.EvidenceConfirmedRunningModel, Detail: "confirmed running model: mistral"}},
			ObservedAt: publishedAt.Add(-time.Second),
		},
	}, recent)

	if !snapshot.PublishedAt.Equal(publishedAt) {
		t.Fatalf("expected published_at %s, got %s", publishedAt, snapshot.PublishedAt)
	}
	if snapshot.Confirmed.Ollama.Version != "0.8.0" {
		t.Fatalf("expected version 0.8.0, got %q", snapshot.Confirmed.Ollama.Version)
	}
	if snapshot.Confirmed.Ollama.PrimaryModel != "mistral" {
		t.Fatalf("expected primary model mistral, got %q", snapshot.Confirmed.Ollama.PrimaryModel)
	}
	if snapshot.Confirmed.System.Process.PID != 4242 {
		t.Fatalf("expected pid 4242, got %d", snapshot.Confirmed.System.Process.PID)
	}
	if snapshot.Confirmed.System.Connections.Count != 1 {
		t.Fatalf("expected one confirmed connection, got %d", snapshot.Confirmed.System.Connections.Count)
	}
	if snapshot.Inferred.Current.Kind != activity.KindInferredModelChanged {
		t.Fatalf("expected current inferred kind %q, got %q", activity.KindInferredModelChanged, snapshot.Inferred.Current.Kind)
	}
	if len(snapshot.Inferred.Recent) != 2 {
		t.Fatalf("expected two recent inferred events, got %d", len(snapshot.Inferred.Recent))
	}
	if len(snapshot.Recent.ConfirmedModels) != 2 || snapshot.Recent.ConfirmedModels[1].Model != "gemma3" {
		t.Fatalf("expected recent confirmed models to preserve history, got %+v", snapshot.Recent.ConfirmedModels)
	}
	if snapshot.Health.Connections.Status != string(system.StatusConfirmed) {
		t.Fatalf("expected confirmed connection health, got %q", snapshot.Health.Connections.Status)
	}
	if snapshot.Passive.ExactRequestLatencyAvailable {
		t.Fatal("expected passive snapshot to keep exact request latency unavailable")
	}
	if snapshot.Passive.ExactStreamingChunksAvailable {
		t.Fatal("expected passive snapshot to keep exact streaming chunks unavailable")
	}
	if len(snapshot.Passive.Notes) != 5 {
		t.Fatalf("expected five passive limitation notes, got %d", len(snapshot.Passive.Notes))
	}
}

func TestProjectorFallsBackToRecentActivityWhenCurrentActivityIsMissing(t *testing.T) {
	t.Parallel()

	publishedAt := time.Date(2026, time.June, 15, 0, 5, 0, 0, time.UTC)
	recent := stubRecentReader{
		activities: []activity.Event{{
			Kind:       activity.KindInferredIdle,
			Truth:      activity.TruthInferred,
			Model:      "gemma3",
			Confidence: activity.ConfidenceMedium,
			ObservedAt: publishedAt.Add(-20 * time.Second),
		}},
	}
	projector := NewProjector(func() time.Time { return publishedAt })

	snapshot := projector.Project(ProjectionInput{
		Ollama: ollama.PollSnapshot{
			Meta:    ollama.SnapshotMeta{ObservedAt: publishedAt.Add(-30 * time.Second), Status: ollama.StatusUnreachable, Reachable: false, Error: "dial tcp"},
			Running: ollama.RunningModelsSnapshot{Meta: ollama.SnapshotMeta{ObservedAt: publishedAt.Add(-30 * time.Second), Status: ollama.StatusUnreachable, Reachable: false, Error: "dial tcp"}},
		},
		System: orchestrator.SystemSnapshot{
			CollectedAt: publishedAt.Add(-15 * time.Second),
			Process:     system.ProcessSnapshot{Meta: system.SnapshotMeta{ObservedAt: publishedAt.Add(-15 * time.Second), Status: system.StatusUnavailable, Healthy: false, Reachable: false, Supported: true, Error: "process unavailable"}},
			Connections: system.ConnectionsSnapshot{Meta: system.SnapshotMeta{ObservedAt: publishedAt.Add(-15 * time.Second), Status: system.StatusUnsupported, Healthy: false, Reachable: false, Supported: false, Error: "pid missing"}},
			Host:        system.HostSnapshot{Meta: system.SnapshotMeta{ObservedAt: publishedAt.Add(-15 * time.Second), Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true}},
		},
	}, recent)

	if snapshot.Inferred.Current.Kind != activity.KindInferredIdle {
		t.Fatalf("expected fallback current activity to use recent idle event, got %q", snapshot.Inferred.Current.Kind)
	}
	if snapshot.Health.Ollama.Status != string(ollama.StatusUnreachable) {
		t.Fatalf("expected unreachable ollama status, got %q", snapshot.Health.Ollama.Status)
	}
	if snapshot.Health.Connections.Status != string(system.StatusUnsupported) {
		t.Fatalf("expected unsupported connection health, got %q", snapshot.Health.Connections.Status)
	}
	if snapshot.Confirmed.Ollama.PrimaryModel != "" {
		t.Fatalf("expected no primary model when no running model exists, got %q", snapshot.Confirmed.Ollama.PrimaryModel)
	}
}

func TestProjectorSnapshotJSONUsesArraysForFrontendCollections(t *testing.T) {
	t.Parallel()

	publishedAt := time.Date(2026, time.June, 17, 0, 0, 0, 0, time.UTC)
	projector := NewProjector(func() time.Time { return publishedAt })

	snapshot := projector.Project(ProjectionInput{
		Capture: CaptureInput{
			SourceActive:    true,
			HasLatency:      true,
			HasTokenCounts:  true,
			HasPayload:      true,
			HasStatus:       true,
			HasStreamChunks: true,
		},
	}, stubRecentReader{})

	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	var decoded struct {
		Inferred struct {
			Current struct {
				Evidence []activity.Evidence `json:"evidence"`
			} `json:"current"`
			Recent []activity.Event `json:"recent"`
		} `json:"inferred"`
		Recent struct {
			ConfirmedModels []RecentConfirmedModel `json:"confirmedModels"`
		} `json:"recent"`
		Inference struct {
			Recent []inference.Inference `json:"recent"`
		} `json:"inference"`
		Passive struct {
			Notes []string `json:"notes"`
		} `json:"passive"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal snapshot payload: %v", err)
	}

	if decoded.Inferred.Current.Evidence == nil {
		t.Fatal("expected inferred.current.evidence to marshal as [] instead of null")
	}
	if decoded.Inferred.Recent == nil {
		t.Fatal("expected inferred.recent to marshal as [] instead of null")
	}
	if decoded.Recent.ConfirmedModels == nil {
		t.Fatal("expected recent.confirmedModels to marshal as [] instead of null")
	}
	if decoded.Inference.Recent == nil {
		t.Fatal("expected inference.recent to marshal as [] instead of null")
	}
	if decoded.Passive.Notes == nil {
		t.Fatal("expected passive.notes to marshal as [] instead of null")
	}
}

func TestPublisherEmitsProjectedSnapshotThroughEmitter(t *testing.T) {
	t.Parallel()

	publishedAt := time.Date(2026, time.June, 15, 0, 10, 0, 0, time.UTC)
	emitter := &stubEmitter{}
	publisher := NewPublisher(
		NewProjector(func() time.Time { return publishedAt }),
		stubRecentReader{},
		emitter,
	)

	snapshot, err := publisher.Publish(context.Background(), ProjectionInput{
		Ollama: ollama.PollSnapshot{
			Meta:    ollama.SnapshotMeta{ObservedAt: publishedAt, Status: ollama.StatusConfirmed, Reachable: true},
			Running: ollama.RunningModelsSnapshot{Models: []ollama.RunningModel{{Name: "phi4"}}},
		},
		System: orchestrator.SystemSnapshot{
			CollectedAt: publishedAt,
			Process:     system.ProcessSnapshot{Meta: system.SnapshotMeta{ObservedAt: publishedAt, Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true}},
			Connections: system.ConnectionsSnapshot{Meta: system.SnapshotMeta{ObservedAt: publishedAt, Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true}},
			Host:        system.HostSnapshot{Meta: system.SnapshotMeta{ObservedAt: publishedAt, Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true}},
		},
		Activity: activity.Event{Kind: activity.KindInferredModelLoaded, Truth: activity.TruthInferred, Model: "phi4", Confidence: activity.ConfidenceHigh, ObservedAt: publishedAt},
	})
	if err != nil {
		t.Fatalf("publish returned error: %v", err)
	}
	if emitter.calls != 1 {
		t.Fatalf("expected one emit call, got %d", emitter.calls)
	}
	if emitter.topic != TopicDashboardSnapshot {
		t.Fatalf("expected topic %q, got %q", TopicDashboardSnapshot, emitter.topic)
	}
	payload, ok := emitter.payload.(Snapshot)
	if !ok {
		t.Fatalf("expected payload type Snapshot, got %T", emitter.payload)
	}
	if payload.Confirmed.Ollama.PrimaryModel != "phi4" || snapshot.Confirmed.Ollama.PrimaryModel != "phi4" {
		t.Fatalf("expected emitted and returned snapshot to include phi4, emitted=%+v returned=%+v", payload.Confirmed.Ollama, snapshot.Confirmed.Ollama)
	}
}

func TestPublisherReturnsEmitterErrorAfterAttemptingSnapshotPublish(t *testing.T) {
	t.Parallel()

	publishedAt := time.Date(2026, time.June, 15, 0, 12, 0, 0, time.UTC)
	emitter := &stubEmitter{err: errors.New("emit failed")}
	publisher := NewPublisher(
		NewProjector(func() time.Time { return publishedAt }),
		stubRecentReader{},
		emitter,
	)

	snapshot, err := publisher.Publish(context.Background(), ProjectionInput{
		Ollama: ollama.PollSnapshot{
			Meta:    ollama.SnapshotMeta{ObservedAt: publishedAt, Status: ollama.StatusConfirmed, Reachable: true},
			Running: ollama.RunningModelsSnapshot{Models: []ollama.RunningModel{{Model: "phi4-mini"}}},
		},
		System: orchestrator.SystemSnapshot{
			CollectedAt: publishedAt,
			Process:     system.ProcessSnapshot{Meta: system.SnapshotMeta{ObservedAt: publishedAt, Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true}},
			Connections: system.ConnectionsSnapshot{Meta: system.SnapshotMeta{ObservedAt: publishedAt, Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true}},
			Host:        system.HostSnapshot{Meta: system.SnapshotMeta{ObservedAt: publishedAt, Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true}},
		},
	})
	if err == nil || err.Error() != "emit failed" {
		t.Fatalf("expected emitter error, got %v", err)
	}
	if !snapshot.PublishedAt.IsZero() || snapshot.Confirmed.Ollama.PrimaryModel != "" || len(snapshot.Passive.Notes) != 0 {
		t.Fatalf("expected publish to return zero-value snapshot on emitter error, got %+v", snapshot)
	}
	if emitter.calls != 1 {
		t.Fatalf("expected one emit attempt, got %d", emitter.calls)
	}
	if emitter.topic != TopicDashboardSnapshot {
		t.Fatalf("expected topic %q, got %q", TopicDashboardSnapshot, emitter.topic)
	}
	payload, ok := emitter.payload.(Snapshot)
	if !ok {
		t.Fatalf("expected snapshot payload, got %T", emitter.payload)
	}
	if payload.Confirmed.Ollama.PrimaryModel != "phi4-mini" {
		t.Fatalf("expected attempted payload to preserve projected model, got %+v", payload.Confirmed.Ollama)
	}
}

// ---- captureMode tests (tasks 4.1 / 4.3 / 4.5 RED) ----------------------------

// CaptureInput carries the per-category data signals used by captureMode to
// decide which PassiveLimitMode flags to flip. A zero-value means capture is
// either disabled or has not produced that category of data yet.
//
// These tests drive the design of captureMode(CaptureInput) PassiveLimitMode,
// which replaces the hardcoded passiveMode() helper.

func TestCaptureMode_FlagsFlipTrueWhenSourceActive(t *testing.T) {
	t.Parallel()

	// All five data categories are present: the active capture source has seen
	// latency, token counts, payload, status, and streaming chunk data.
	input := CaptureInput{
		SourceActive:    true,
		HasLatency:      true,
		HasTokenCounts:  true,
		HasPayload:      true,
		HasStatus:       true,
		HasStreamChunks: true,
	}

	got := captureMode(input)

	if !got.ExactRequestLatencyAvailable {
		t.Error("expected ExactRequestLatencyAvailable=true when capture provides latency")
	}
	if !got.ExactTokenCountsAvailable {
		t.Error("expected ExactTokenCountsAvailable=true when capture provides token counts")
	}
	if !got.ExactPayloadAvailable {
		t.Error("expected ExactPayloadAvailable=true when capture provides payload")
	}
	if !got.ExactStatusAvailable {
		t.Error("expected ExactStatusAvailable=true when capture provides status")
	}
	if !got.ExactStreamingChunksAvailable {
		t.Error("expected ExactStreamingChunksAvailable=true when capture provides streaming chunks")
	}
	if got.Mode != "capture-active" {
		t.Errorf("expected mode=capture-active, got %q", got.Mode)
	}
}

func TestCaptureMode_DisabledOrUnelevatedStaysHonest(t *testing.T) {
	t.Parallel()

	// Capture is disabled / unelevated — no data available at all.
	input := CaptureInput{
		SourceActive:   false,
		UnelevatedNote: "run as administrator to enable live capture",
	}

	got := captureMode(input)

	if got.ExactRequestLatencyAvailable {
		t.Error("expected ExactRequestLatencyAvailable=false when source inactive")
	}
	if got.ExactTokenCountsAvailable {
		t.Error("expected ExactTokenCountsAvailable=false when source inactive")
	}
	if got.ExactPayloadAvailable {
		t.Error("expected ExactPayloadAvailable=false when source inactive")
	}
	if got.ExactStatusAvailable {
		t.Error("expected ExactStatusAvailable=false when source inactive")
	}
	if got.ExactStreamingChunksAvailable {
		t.Error("expected ExactStreamingChunksAvailable=false when source inactive")
	}
	if len(got.Notes) == 0 {
		t.Error("expected at least one Note explaining capture unavailability")
	}
	found := false
	for _, note := range got.Notes {
		if note == "run as administrator to enable live capture" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unelevated note in Notes, got %v", got.Notes)
	}
}

func TestCaptureMode_PartialDataPerCategoryHonesty(t *testing.T) {
	t.Parallel()

	// Source is active, has status and payload, but this exchange was a
	// /api/tags call — no token counts or latency or streaming chunks.
	input := CaptureInput{
		SourceActive:    true,
		HasLatency:      false,
		HasTokenCounts:  false,
		HasPayload:      true,
		HasStatus:       true,
		HasStreamChunks: false,
	}

	got := captureMode(input)

	if got.ExactTokenCountsAvailable {
		t.Error("expected ExactTokenCountsAvailable=false for metadata-only exchange lacking eval_count")
	}
	if got.ExactRequestLatencyAvailable {
		t.Error("expected ExactRequestLatencyAvailable=false for exchange without latency data")
	}
	if !got.ExactStatusAvailable {
		t.Error("expected ExactStatusAvailable=true when status data is present")
	}
	if !got.ExactPayloadAvailable {
		t.Error("expected ExactPayloadAvailable=true when payload data is present")
	}
	if got.ExactStreamingChunksAvailable {
		t.Error("expected ExactStreamingChunksAvailable=false for non-streaming exchange")
	}
}

// ---- running model enrichment tests (task 4.7 RED) ---------------------------

func TestConfirmedOllama_RunningModelEnrichmentPassthrough(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 6, 16, 13, 0, 0, 0, time.UTC)
	snap := ollama.PollSnapshot{
		Meta: ollama.SnapshotMeta{Status: ollama.StatusConfirmed, Reachable: true},
		Running: ollama.RunningModelsSnapshot{
			Meta: ollama.SnapshotMeta{Status: ollama.StatusConfirmed},
			Models: []ollama.RunningModel{
				{
					Name:          "llama3:8b-q4_0",
					Model:         "llama3",
					Size:          4_500_000_000,
					SizeVRAM:      4_200_000_000,
					Details:       ollama.ModelDetails{ParameterSize: "8B", QuantizationLevel: "Q4_0"},
					ContextLength: 8192,
					ExpiresAt:     expiresAt,
				},
			},
		},
	}

	confirmed := confirmedOllama(snap)

	if len(confirmed.RunningModels) != 1 || confirmed.RunningModels[0] != "llama3:8b-q4_0" {
		t.Fatalf("expected legacy RunningModels to preserve string name, got %v", confirmed.RunningModels)
	}
	if len(confirmed.RunningModelDetails) != 1 {
		t.Fatalf("expected RunningModelDetails to have 1 entry, got %d", len(confirmed.RunningModelDetails))
	}
	d := confirmed.RunningModelDetails[0]
	if d.Name != "llama3:8b-q4_0" {
		t.Errorf("expected Name=llama3:8b-q4_0, got %q", d.Name)
	}
	if d.Size != 4_500_000_000 {
		t.Errorf("expected Size=4500000000, got %d", d.Size)
	}
	if d.SizeVRAM != 4_200_000_000 {
		t.Errorf("expected SizeVRAM=4200000000, got %d", d.SizeVRAM)
	}
	if d.ParameterSize != "8B" {
		t.Errorf("expected ParameterSize=8B, got %q", d.ParameterSize)
	}
	if d.QuantizationLevel != "Q4_0" {
		t.Errorf("expected QuantizationLevel=Q4_0, got %q", d.QuantizationLevel)
	}
	if d.ContextLength != 8192 {
		t.Errorf("expected ContextLength=8192, got %d", d.ContextLength)
	}
	if !d.ExpiresAt.Equal(expiresAt) {
		t.Errorf("expected ExpiresAt=%v, got %v", expiresAt, d.ExpiresAt)
	}
}

type stubRecentReader struct {
	snapshots  []store.Snapshot
	activities []activity.Event
}

func (reader stubRecentReader) Snapshots() []store.Snapshot {
	return append([]store.Snapshot(nil), reader.snapshots...)
}

func (reader stubRecentReader) Activities() []activity.Event {
	return append([]activity.Event(nil), reader.activities...)
}

type stubEmitter struct {
	calls   int
	topic   string
	payload any
	err     error
}

func (emitter *stubEmitter) Emit(_ context.Context, topic string, payload any) error {
	emitter.calls++
	emitter.topic = topic
	emitter.payload = payload
	return emitter.err
}
