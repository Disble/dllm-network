package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"ollama-telemetry/internal/capture"
	"ollama-telemetry/internal/dashboard"
	"ollama-telemetry/internal/telemetry"
	"ollama-telemetry/internal/telemetry/orchestrator"
)

// TestDependencies_CaptureSourceNilDefaultsToNoop asserts that NewWithDependencies
// with nil CaptureSource falls back to the noop source (never panics, runs without
// elevation, status shows inactive).
func TestDependencies_CaptureSourceNilDefaultsToNoop(t *testing.T) {
	t.Parallel()

	app := NewWithDependencies(Dependencies{
		Window:       &fakeWindow{},
		Orchestrator: &fakeOrchestrator{state: orchestrator.StateRunning},
		Config:       telemetry.Config{ShutdownTimeout: time.Second},
	})

	st := app.captureSource.Status()
	if st.Active {
		t.Fatalf("expected nil CaptureSource to default to noop (inactive), got Active=true reason=%q", st.Reason)
	}
}

// TestDependencies_CaptureSourceInjected asserts that a CaptureSource provided
// in Dependencies is wired directly (not replaced by noop).
func TestDependencies_CaptureSourceInjected(t *testing.T) {
	t.Parallel()

	fake := capture.NewFakeSource(nil)
	_ = fake.Open()

	app := NewWithDependencies(Dependencies{
		Window:         &fakeWindow{},
		Orchestrator:   &fakeOrchestrator{state: orchestrator.StateRunning},
		Config:         telemetry.Config{ShutdownTimeout: time.Second},
		CaptureSource:  fake,
	})

	st := app.captureSource.Status()
	if !st.Active {
		t.Fatalf("expected injected FakeSource to report Active=true, got Active=false reason=%q", st.Reason)
	}
}

// TestNewCaptureSourceSeam asserts that the package-level newCaptureSource
// func-var is callable and returns a non-nil CaptureSource. In tests it is
// replaced via Dependencies injection; this test verifies the seam exists.
func TestNewCaptureSourceSeam(t *testing.T) {
	t.Parallel()

	src := newCaptureSource()
	if src == nil {
		t.Fatal("expected newCaptureSource func-var to return a non-nil CaptureSource")
	}
}

// TestUnelevatedDegradation_NoopSourceEmitsUnavailableSnapshot asserts that
// when the capture source is not active (noop/unelevated), Startup does not
// crash and the pipeline loop exits cleanly, and the app remains operational.
func TestUnelevatedDegradation_StartupWithNoopSource(t *testing.T) {
	t.Parallel()

	app := NewWithDependencies(Dependencies{
		Window:       &fakeWindow{},
		Orchestrator: &fakeOrchestrator{state: orchestrator.StateRunning},
		Config:       telemetry.Config{ShutdownTimeout: 100 * time.Millisecond},
	})

	ctx, cancel := context.WithCancel(context.Background())
	app.Startup(ctx)

	// App must remain responsive (not deadlocked or panicked).
	if got := app.Status(); got.CollectionState == "" {
		t.Fatalf("expected app to report a collection state, got empty string")
	}

	cancel()
	if err := app.Quit(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("quit after noop-source startup failed: %v", err)
	}
}

// TestCapturePipeline_FakeSourceSegmentsReachEmitter asserts the entire pipeline:
// fakeSource → reassembly → httpx → inference → emitted Snapshot.Inference.
// Uses golden segments containing a /api/generate exchange.
func TestCapturePipeline_FakeSourceSegmentsReachEmitter(t *testing.T) {
	if testing.Short() {
		t.Skip("pipeline integration test skipped in short mode")
	}

	// Build a minimal /api/generate request + streamed NDJSON response that
	// the inference extractor will classify as PhaseCompleted.
	// We use a Content-Length request and a chunked NDJSON response.
	reqBytes := buildFakeGenerateRequest()
	respBytes := buildFakeGenerateResponse()

	// Combine into a single connection: request goes ToServer, response FromServer.
	// SeqNo=0 for in-order delivery.
	baseTime := time.Now()
	tuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 54321, DstPort: 11434}

	segments := []capture.Segment{
		{Tuple: tuple, Dir: capture.DirToServer, Payload: reqBytes, SeqNo: 0, At: baseTime},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: respBytes, SeqNo: 0, At: baseTime.Add(time.Millisecond)},
	}

	fake := capture.NewFakeSource(segments)

	// Collect emitted snapshots.
	var emitted []dashboard.Snapshot
	emitFn := func(ctx context.Context, event string, payload ...any) {
		if event == dashboard.TopicDashboardSnapshot {
			if snap, ok := payload[0].(dashboard.Snapshot); ok {
				emitted = append(emitted, snap)
			}
		}
	}

	app := newTestAppWithEmitter(fake, emitFn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	app.Startup(ctx)

	// Wait until at least one snapshot with a completed inference is emitted.
	deadline := time.Now().Add(1500 * time.Millisecond)
	var foundInference bool
	for time.Now().Before(deadline) {
		for _, snap := range emitted {
			if snap.Inference.Current.Status == 1 { // PhaseCompleted == 1
				foundInference = true
				// PassiveLimitMode should reflect capture-active.
				if snap.Passive.Mode != "capture-active" {
					t.Errorf("expected passive mode 'capture-active', got %q", snap.Passive.Mode)
				}
				break
			}
		}
		if foundInference {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !foundInference {
		t.Fatalf("no emitted snapshot with PhaseCompleted inference within timeout; got %d snapshots", len(emitted))
	}
}

// TestCapturePipeline_QuitCancelsCapture asserts that Quit cancels the capture
// goroutine context and Close is called on the source, within ShutdownTimeout.
func TestCapturePipeline_QuitCancelsCapture(t *testing.T) {
	t.Parallel()

	trackingSource := &trackingCaptureSource{inner: capture.NewNoopSource()}

	app := NewWithDependencies(Dependencies{
		Window:        &fakeWindow{},
		Orchestrator:  &fakeOrchestrator{state: orchestrator.StateRunning},
		Config:        telemetry.Config{ShutdownTimeout: 200 * time.Millisecond},
		CaptureSource: trackingSource,
	})

	app.Startup(context.Background())
	time.Sleep(20 * time.Millisecond) // let goroutine start

	if err := app.Quit(); err != nil {
		t.Fatalf("quit: %v", err)
	}

	if !trackingSource.closed {
		t.Fatal("expected capture source Close() to be called on Quit")
	}
}

// ---- helpers ----------------------------------------------------------------

// newTestAppWithEmitter creates an App wired with the fakeSource as CaptureSource
// and a custom emit func so tests can capture emitted snapshots.
func newTestAppWithEmitter(src capture.CaptureSource, emitFn runtimeEventEmitter) *App {
	emitter := wailsEmitter{emit: emitFn}
	return newTestAppWithEmitterAndSource(src, emitter)
}

func newTestAppWithEmitterAndSource(src capture.CaptureSource, emitter wailsEmitter) *App {
	return NewWithDependencies(Dependencies{
		Window:        &fakeWindow{},
		Orchestrator:  &fakeOrchestrator{state: orchestrator.StateRunning},
		Config:        telemetry.Config{ShutdownTimeout: 500 * time.Millisecond},
		CaptureSource: src,
		wailsEmitter:  emitter,
	})
}

// trackingCaptureSource wraps a CaptureSource and records whether Close was called.
type trackingCaptureSource struct {
	inner  capture.CaptureSource
	closed bool
}

func (t *trackingCaptureSource) Open() error                              { return t.inner.Open() }
func (t *trackingCaptureSource) Recv(ctx context.Context) (capture.Segment, error) {
	return t.inner.Recv(ctx)
}
func (t *trackingCaptureSource) Close() error {
	t.closed = true
	return t.inner.Close()
}
func (t *trackingCaptureSource) Status() capture.SourceStatus { return t.inner.Status() }

// buildFakeGenerateRequest produces a minimal HTTP POST /api/generate request.
func buildFakeGenerateRequest() []byte {
	body := `{"model":"llama3","prompt":"hello"}`
	return []byte("POST /api/generate HTTP/1.1\r\nHost: 127.0.0.1:11434\r\nContent-Type: application/json\r\nContent-Length: " +
		itoa(len(body)) + "\r\n\r\n" + body)
}

// buildFakeGenerateResponse produces a minimal chunked NDJSON response with
// one in-progress chunk and one done:true terminal chunk.
func buildFakeGenerateResponse() []byte {
	line1 := `{"model":"llama3","response":"hi","done":false}`
	line2 := `{"model":"llama3","response":"","done":true,"prompt_eval_count":5,"eval_count":3,"eval_duration":300000000,"total_duration":350000000,"load_duration":10000000}`

	chunk1 := itohex(len(line1)+1) + "\r\n" + line1 + "\n" + "\r\n"
	chunk2 := itohex(len(line2)+1) + "\r\n" + line2 + "\n" + "\r\n"
	terminal := "0\r\n\r\n"

	return []byte("HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\nContent-Type: application/x-ndjson\r\n\r\n" +
		chunk1 + chunk2 + terminal)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func itohex(n int) string {
	const hexChars = "0123456789abcdef"
	if n == 0 {
		return "0"
	}
	result := make([]byte, 0, 4)
	for n > 0 {
		result = append([]byte{hexChars[n&0xf]}, result...)
		n >>= 4
	}
	return string(result)
}
