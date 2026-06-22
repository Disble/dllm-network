package app

import (
	"context"
	"sync"

	"dllm-network/internal/activity"
	"dllm-network/internal/dashboard"
	"dllm-network/internal/telemetry/ollama"
	"dllm-network/internal/telemetry/orchestrator"
)

// snapshotAssembler is the single shared state that prevents the two emitter
// paths (orchestrator tick publisher + capture pipeline publisher) from
// clobbering each other on the "dashboard:snapshot" Wails event.
//
// Problem: the orchestrator tick emits a Snapshot with Ollama/System populated
// but Inference empty; the capture pipeline emits a Snapshot with
// Inference/Capture populated but Ollama/System empty. Both publish to the same
// event name and the frontend REPLACES its cached snapshot on receipt — so the
// two partial snapshots alternate/clobber each other in the UI.
//
// Fix: both paths write their partial state into the assembler, which keeps the
// latest contribution from each side and emits the COMBINED Snapshot through the
// shared dashboard.Publisher. Every emitted "dashboard:snapshot" is therefore
// complete — it carries the latest Ollama+System data AND the latest
// Inference+Capture data.
//
// The assembler is concurrency-safe: the orchestrator tick goroutine and the
// capture pipeline goroutine write from different goroutines. A single mutex
// serialises all updates and the downstream Publish call.
type snapshotAssembler struct {
	mu        sync.Mutex
	publisher snapshotProjector

	// Last Ollama/System state from the orchestrator tick path.
	lastOllama   ollama.PollSnapshot
	lastSystem   orchestrator.SystemSnapshot
	lastActivity activity.Event

	// Last Inference/Capture state from the capture pipeline path.
	lastInference dashboard.InferenceState
	lastCapture   dashboard.CaptureInput
}

// newSnapshotAssembler creates an assembler that funnels all Publish calls
// through the provided publisher. The publisher already holds the emitter and
// the recent store — no additional wiring is needed.
func newSnapshotAssembler(publisher snapshotProjector) *snapshotAssembler {
	return &snapshotAssembler{publisher: publisher}
}

// PublishOllamaSystem is called by the orchestrator tick path. It updates the
// Ollama/System side of the shared state and emits a merged Snapshot.
func (a *snapshotAssembler) PublishOllamaSystem(ctx context.Context, input dashboard.ProjectionInput) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lastOllama = input.Ollama
	a.lastSystem = input.System
	a.lastActivity = input.Activity

	_, err := a.publisher.Publish(ctx, a.merged())
	return err
}

// PublishCapture is called by the capture pipeline path. It updates the
// Capture/Inference side of the shared state and emits a merged Snapshot.
func (a *snapshotAssembler) PublishCapture(ctx context.Context, input dashboard.ProjectionInput) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lastInference = input.Inference
	a.lastCapture = input.Capture

	_, err := a.publisher.Publish(ctx, a.merged())
	return err
}

// merged builds the combined ProjectionInput from both sides' latest state.
// Must be called with a.mu held.
func (a *snapshotAssembler) merged() dashboard.ProjectionInput {
	return dashboard.ProjectionInput{
		Ollama:    a.lastOllama,
		System:    a.lastSystem,
		Activity:  a.lastActivity,
		Capture:   a.lastCapture,
		Inference: a.lastInference,
	}
}

// OllamaSystemPublisher wraps a snapshotAssembler so it satisfies the
// snapshotProjector interface expected by runtimePublisher. Every Publish call
// routes through PublishOllamaSystem, updating only the Ollama/System side of
// the shared state before emitting the merged Snapshot.
type ollamaSystemPublisher struct {
	assembler *snapshotAssembler
}

func (p ollamaSystemPublisher) Publish(ctx context.Context, input dashboard.ProjectionInput) (dashboard.Snapshot, error) {
	err := p.assembler.PublishOllamaSystem(ctx, input)
	return dashboard.Snapshot{}, err
}

// capturePublisher wraps a snapshotAssembler so it satisfies the
// snapshotProjector interface expected by inferencePipeline. Every Publish call
// routes through PublishCapture, updating only the Capture/Inference side of
// the shared state before emitting the merged Snapshot.
type capturePublisher struct {
	assembler *snapshotAssembler
}

func (p capturePublisher) Publish(ctx context.Context, input dashboard.ProjectionInput) (dashboard.Snapshot, error) {
	err := p.assembler.PublishCapture(ctx, input)
	return dashboard.Snapshot{}, err
}
