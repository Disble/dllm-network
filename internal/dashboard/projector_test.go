package dashboard

import (
	"context"
	"testing"
	"time"

	"ollama-telemetry/internal/activity"
	"ollama-telemetry/internal/store"
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
}

func (emitter *stubEmitter) Emit(_ context.Context, topic string, payload any) error {
	emitter.calls++
	emitter.topic = topic
	emitter.payload = payload
	return nil
}
