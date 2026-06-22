package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"dllm-network/internal/dashboard"
	"dllm-network/internal/store"
	"dllm-network/internal/telemetry/inference"
	"dllm-network/internal/telemetry/ollama"
	"dllm-network/internal/telemetry/orchestrator"
)

// snapshotCollector wraps a wailsEmitter (dashboard.Emitter) and collects
// emitted dashboard Snapshots, thread-safe. Used in assembler tests to observe
// what the assembler emits without a live Wails runtime.
type snapshotCollector struct {
	mu        sync.Mutex
	snapshots []dashboard.Snapshot
}

func (e *snapshotCollector) Emit(_ context.Context, event string, payload any) error {
	if event != dashboard.TopicDashboardSnapshot {
		return nil
	}
	snap, ok := payload.(dashboard.Snapshot)
	if !ok {
		return nil
	}
	e.mu.Lock()
	e.snapshots = append(e.snapshots, snap)
	e.mu.Unlock()
	return nil
}

func (e *snapshotCollector) last() (dashboard.Snapshot, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.snapshots) == 0 {
		return dashboard.Snapshot{}, false
	}
	return e.snapshots[len(e.snapshots)-1], true
}

// newTestAssembler creates a snapshotAssembler backed by a snapshotCollector so
// tests can assert on emitted Snapshots without a live Wails runtime.
func newTestAssembler(col *snapshotCollector) *snapshotAssembler {
	recent := store.NewRecent(12, 12)
	pub := dashboard.NewPublisher(nil, recent, col)
	return newSnapshotAssembler(pub)
}

// TestSnapshotAssembler_OrchestratorThenCapture_BothPopulated is the RED test.
//
// It drives two separate Publish calls:
//  1. An orchestrator-style call: Ollama/System populated, Inference zero.
//  2. A capture-style call:       Inference populated, Ollama/System zero.
//
// Without the shared assembler, the last emitted Snapshot would have Inference
// populated but Ollama/System empty — clobbering. With the fix every emitted
// Snapshot must carry BOTH.
func TestSnapshotAssembler_OrchestratorThenCapture_BothPopulated(t *testing.T) {
	t.Parallel()

	col := &snapshotCollector{}
	assembler := newTestAssembler(col)

	ctx := context.Background()

	// Step 1 — orchestrator tick: Ollama/System present, Inference absent.
	ollamaInput := ollama.PollSnapshot{}
	ollamaInput.Running.Models = []ollama.RunningModel{{Name: "llama3"}}
	sysInput := orchestrator.SystemSnapshot{CollectedAt: time.Now()}

	err := assembler.PublishOllamaSystem(ctx, dashboard.ProjectionInput{
		Ollama:  ollamaInput,
		System:  sysInput,
	})
	if err != nil {
		t.Fatalf("PublishOllamaSystem: %v", err)
	}

	// Verify first emission contains Ollama data.
	snap, ok := col.last()
	if !ok {
		t.Fatal("expected at least one snapshot after orchestrator publish")
	}
	if len(snap.Confirmed.Ollama.RunningModels) == 0 {
		t.Fatalf("after orchestrator publish: expected RunningModels to be present, got empty")
	}
	if snap.Confirmed.Ollama.RunningModels[0] != "llama3" {
		t.Fatalf("after orchestrator publish: expected RunningModels[0]='llama3', got %q", snap.Confirmed.Ollama.RunningModels[0])
	}

	// Step 2 — capture pipeline: Inference present, Ollama/System absent.
	inf := inference.Inference{
		Model:  "llama3",
		Status: inference.PhaseCompleted,
	}
	inferenceState := dashboard.InferenceState{
		Current: inf,
		Recent:  []inference.Inference{inf},
	}

	err = assembler.PublishCapture(ctx, dashboard.ProjectionInput{
		Inference: inferenceState,
		Capture: dashboard.CaptureInput{
			SourceActive: true,
			HasStatus:    true,
		},
	})
	if err != nil {
		t.Fatalf("PublishCapture: %v", err)
	}

	// The LAST emitted snapshot must carry BOTH Ollama and Inference data.
	snap, ok = col.last()
	if !ok {
		t.Fatal("expected snapshot after capture publish")
	}

	if len(snap.Confirmed.Ollama.RunningModels) == 0 || snap.Confirmed.Ollama.RunningModels[0] != "llama3" {
		t.Fatalf("after capture publish: Ollama data was CLOBBERED — expected RunningModels=['llama3'], got %v",
			snap.Confirmed.Ollama.RunningModels)
	}
	if snap.Inference.Current.Model != "llama3" {
		t.Fatalf("after capture publish: Inference data absent — expected Current.Model='llama3', got %q",
			snap.Inference.Current.Model)
	}
}

// TestSnapshotAssembler_CaptureThenOrchestrator_BothPopulated tests the REVERSE
// order: capture publishes first, then orchestrator publishes — the last emitted
// Snapshot must still contain BOTH.
func TestSnapshotAssembler_CaptureThenOrchestrator_BothPopulated(t *testing.T) {
	t.Parallel()

	col := &snapshotCollector{}
	assembler := newTestAssembler(col)

	ctx := context.Background()

	// Step 1 — capture pipeline publishes first.
	inf := inference.Inference{
		Model:  "gemma2",
		Status: inference.PhaseCompleted,
	}
	inferenceState := dashboard.InferenceState{
		Current: inf,
		Recent:  []inference.Inference{inf},
	}

	err := assembler.PublishCapture(ctx, dashboard.ProjectionInput{
		Inference: inferenceState,
		Capture: dashboard.CaptureInput{
			SourceActive: true,
			HasStatus:    true,
		},
	})
	if err != nil {
		t.Fatalf("PublishCapture: %v", err)
	}

	// Step 2 — orchestrator tick with Ollama/System data.
	ollamaInput := ollama.PollSnapshot{}
	ollamaInput.Running.Models = []ollama.RunningModel{{Name: "gemma2"}}
	sysInput := orchestrator.SystemSnapshot{CollectedAt: time.Now()}

	err = assembler.PublishOllamaSystem(ctx, dashboard.ProjectionInput{
		Ollama:  ollamaInput,
		System:  sysInput,
	})
	if err != nil {
		t.Fatalf("PublishOllamaSystem: %v", err)
	}

	// The LAST emitted snapshot must carry BOTH.
	snap, ok := col.last()
	if !ok {
		t.Fatal("expected snapshot after orchestrator publish")
	}

	if len(snap.Confirmed.Ollama.RunningModels) == 0 || snap.Confirmed.Ollama.RunningModels[0] != "gemma2" {
		t.Fatalf("after orchestrator publish (reverse order): expected RunningModels=['gemma2'], got %v",
			snap.Confirmed.Ollama.RunningModels)
	}
	if snap.Inference.Current.Model != "gemma2" {
		t.Fatalf("after orchestrator publish (reverse order): Inference data was CLOBBERED — expected Current.Model='gemma2', got %q",
			snap.Inference.Current.Model)
	}
}

// TestSnapshotAssembler_ConcurrentPublish_NoPanic verifies that concurrent calls
// from both paths do not race or panic. The content need not be deterministic but
// no snapshot may be partially zeroed in a way that panics.
func TestSnapshotAssembler_ConcurrentPublish_NoPanic(t *testing.T) {
	t.Parallel()

	col := &snapshotCollector{}
	assembler := newTestAssembler(col)

	ctx := context.Background()
	var wg sync.WaitGroup
	const rounds = 50

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			ollamaInput := ollama.PollSnapshot{}
			ollamaInput.Running.Models = []ollama.RunningModel{{Name: "llama3"}}
			_ = assembler.PublishOllamaSystem(ctx, dashboard.ProjectionInput{
				Ollama: ollamaInput,
				System: orchestrator.SystemSnapshot{CollectedAt: time.Now()},
			})
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			inf := inference.Inference{Model: "llama3", Status: inference.PhaseCompleted}
			_ = assembler.PublishCapture(ctx, dashboard.ProjectionInput{
				Inference: dashboard.InferenceState{Current: inf},
				Capture:   dashboard.CaptureInput{SourceActive: true},
			})
		}
	}()

	wg.Wait()
}
