package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"ollama-telemetry/internal/capture"
	"ollama-telemetry/internal/dashboard"
	"ollama-telemetry/internal/telemetry"
	"ollama-telemetry/internal/telemetry/inference"
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

// TestCapturePipeline_AssemblesDetailFields asserts the pipeline surfaces the
// DevTools-Network detail fields: a stable id, the captured request body and
// headers, the response status code, and the response body ASSEMBLED across the
// streamed NDJSON lines (not just the terminal line).
func TestCapturePipeline_AssemblesDetailFields(t *testing.T) {
	if testing.Short() {
		t.Skip("pipeline integration test skipped in short mode")
	}

	baseTime := time.Now()
	tuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 54321, DstPort: 11434}
	segments := []capture.Segment{
		{Tuple: tuple, Dir: capture.DirToServer, Payload: buildFakeGenerateRequest(), SeqNo: 0, At: baseTime},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: buildFakeGenerateResponse(), SeqNo: 0, At: baseTime.Add(time.Millisecond)},
	}

	var emitted []dashboard.Snapshot
	emitFn := func(ctx context.Context, event string, payload ...any) {
		if event == dashboard.TopicDashboardSnapshot {
			if snap, ok := payload[0].(dashboard.Snapshot); ok {
				emitted = append(emitted, snap)
			}
		}
	}

	app := newTestAppWithEmitter(capture.NewFakeSource(segments), emitFn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	app.Startup(ctx)

	deadline := time.Now().Add(1500 * time.Millisecond)
	var done *dashboard.Snapshot
	for time.Now().Before(deadline) && done == nil {
		for i := range emitted {
			if emitted[i].Inference.Current.Status == 1 { // PhaseCompleted
				done = &emitted[i]
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if done == nil {
		t.Fatalf("no completed inference within timeout; got %d snapshots", len(emitted))
	}

	cur := done.Inference.Current
	if cur.ID == "" {
		t.Error("expected a stable non-empty inference id")
	}
	if cur.StatusCode != 200 {
		t.Errorf("StatusCode: got %d, want 200", cur.StatusCode)
	}
	if !strings.Contains(cur.RequestBody, "llama3") {
		t.Errorf("RequestBody should contain the prompt JSON, got %q", cur.RequestBody)
	}
	if !hasHeader(cur.RequestHeaders, "Content-Type") {
		t.Errorf("RequestHeaders missing Content-Type: %+v", cur.RequestHeaders)
	}
	// Assembled response must include BOTH the streamed line and the terminal line.
	if !strings.Contains(cur.ResponseBody, `"response":"hi"`) {
		t.Errorf("ResponseBody missing the streamed line, got %q", cur.ResponseBody)
	}
	if !strings.Contains(cur.ResponseBody, `"done":true`) {
		t.Errorf("ResponseBody missing the terminal line, got %q", cur.ResponseBody)
	}
}

func hasHeader(headers []inference.Header, name string) bool {
	for _, h := range headers {
		if h.Name == name {
			return true
		}
	}
	return false
}

// TestCapturePipeline_PairsAcrossSwappedTuples asserts that a request and its
// response are paired even though the OS delivers them with SWAPPED 4-tuples
// (request: client→server; response: server→client). This models real WinDivert
// capture (verified live in WU5), unlike the same-tuple golden fixture.
func TestCapturePipeline_PairsAcrossSwappedTuples(t *testing.T) {
	if testing.Short() {
		t.Skip("pipeline integration test skipped in short mode")
	}

	reqBytes := buildFakeGenerateRequest()
	respBytes := buildFakeGenerateResponse()

	baseTime := time.Now()
	reqTuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 54321, DstPort: 11434}
	// Response arrives with src/dst SWAPPED — exactly how the OS reports the
	// reverse direction of the same TCP connection.
	respTuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 11434, DstPort: 54321}

	segments := []capture.Segment{
		{Tuple: reqTuple, Dir: capture.DirToServer, Payload: reqBytes, SeqNo: 0, At: baseTime},
		{Tuple: respTuple, Dir: capture.DirFromServer, Payload: respBytes, SeqNo: 0, At: baseTime.Add(time.Millisecond)},
	}

	fake := capture.NewFakeSource(segments)

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

	deadline := time.Now().Add(1500 * time.Millisecond)
	var foundInference bool
	for time.Now().Before(deadline) && !foundInference {
		for _, snap := range emitted {
			if snap.Inference.Current.Status == 1 { // PhaseCompleted
				foundInference = true
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !foundInference {
		t.Fatalf("request/response not paired across swapped tuples; got %d snapshots, no PhaseCompleted inference", len(emitted))
	}
}

// TestCapturePipeline_IgnoresMetadataOnlyPolls asserts that a non-inference
// exchange (e.g. a GET /api/tags poll — including this app's own orchestrator
// polling) never becomes the displayed current inference.
func TestCapturePipeline_IgnoresMetadataOnlyPolls(t *testing.T) {
	if testing.Short() {
		t.Skip("pipeline integration test skipped in short mode")
	}

	getReq := []byte("GET /api/tags HTTP/1.1\r\nHost: 127.0.0.1:11434\r\n\r\n")
	getResp := []byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}")

	baseTime := time.Now()
	reqTuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 55000, DstPort: 11434}
	respTuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 11434, DstPort: 55000}

	segments := []capture.Segment{
		{Tuple: reqTuple, Dir: capture.DirToServer, Payload: getReq, SeqNo: 0, At: baseTime},
		{Tuple: respTuple, Dir: capture.DirFromServer, Payload: getResp, SeqNo: 0, At: baseTime.Add(time.Millisecond)},
	}

	fake := capture.NewFakeSource(segments)

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

	time.Sleep(300 * time.Millisecond)

	for _, snap := range emitted {
		if snap.Inference.Current.Endpoint == "/api/tags" {
			t.Fatal("metadata-only poll incorrectly became the current inference")
		}
	}
}

// TestCapturePipeline_EmitsInProgressOnRequest asserts that an inference row
// appears the moment the REQUEST is observed — before any response arrives.
// This is the stream:false case the user hit: a generation can take many
// seconds during which the server sends nothing, so a response-only trigger
// leaves the row invisible until completion. The in-progress event must carry
// the model but MUST NOT fabricate token metrics (Tokens stays nil).
func TestCapturePipeline_EmitsInProgressOnRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("pipeline integration test skipped in short mode")
	}

	baseTime := time.Now()
	tuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 54321, DstPort: 11434}
	// Only the REQUEST segment — the response has not come back yet.
	segments := []capture.Segment{
		{Tuple: tuple, Dir: capture.DirToServer, Payload: buildFakeGenerateRequest(), SeqNo: 0, At: baseTime},
	}

	var emitted []dashboard.Snapshot
	emitFn := func(ctx context.Context, event string, payload ...any) {
		if event == dashboard.TopicDashboardSnapshot {
			if snap, ok := payload[0].(dashboard.Snapshot); ok {
				emitted = append(emitted, snap)
			}
		}
	}

	app := newTestAppWithEmitter(capture.NewFakeSource(segments), emitFn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	app.Startup(ctx)

	deadline := time.Now().Add(1500 * time.Millisecond)
	var inProgress *dashboard.Snapshot
	for time.Now().Before(deadline) && inProgress == nil {
		for i := range emitted {
			cur := emitted[i].Inference.Current
			if cur.Endpoint == "/api/generate" && cur.Status == inference.PhaseInProgress {
				inProgress = &emitted[i]
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	if inProgress == nil {
		t.Fatalf("expected an in-progress inference emitted on request observation (before any response); got %d snapshots", len(emitted))
	}
	cur := inProgress.Inference.Current
	if cur.Model != "llama3" {
		t.Errorf("in-progress model: got %q, want llama3", cur.Model)
	}
	if cur.Tokens != nil {
		t.Error("in-progress inference must not fabricate token metrics (Tokens must be nil)")
	}
}

// TestCapturePipeline_OpenAIChatCompletions asserts the pipeline captures a
// streamed POST /v1/chat/completions (Ollama's OpenAI-compatible API, SSE
// transport) end to end: it must NOT be filtered as metadata-only, it must
// complete on the [DONE] sentinel, and token counts must come from the SSE
// `usage` chunk in the assembled body.
func TestCapturePipeline_OpenAIChatCompletions(t *testing.T) {
	if testing.Short() {
		t.Skip("pipeline integration test skipped in short mode")
	}

	baseTime := time.Now()
	tuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 54321, DstPort: 11434}
	segments := []capture.Segment{
		{Tuple: tuple, Dir: capture.DirToServer, Payload: buildFakeOpenAIRequest(), SeqNo: 0, At: baseTime},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: buildFakeOpenAIResponse(), SeqNo: 0, At: baseTime.Add(time.Millisecond)},
	}

	var emitted []dashboard.Snapshot
	emitFn := func(ctx context.Context, event string, payload ...any) {
		if event == dashboard.TopicDashboardSnapshot {
			if snap, ok := payload[0].(dashboard.Snapshot); ok {
				emitted = append(emitted, snap)
			}
		}
	}

	app := newTestAppWithEmitter(capture.NewFakeSource(segments), emitFn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	app.Startup(ctx)

	deadline := time.Now().Add(1500 * time.Millisecond)
	var done *dashboard.Snapshot
	for time.Now().Before(deadline) && done == nil {
		for i := range emitted {
			cur := emitted[i].Inference.Current
			if cur.Endpoint == "/v1/chat/completions" && cur.Status == 1 { // PhaseCompleted
				done = &emitted[i]
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if done == nil {
		t.Fatalf("no completed /v1/chat/completions inference; got %d snapshots", len(emitted))
	}

	cur := done.Inference.Current
	if cur.Model != "gemma4:12b" {
		t.Errorf("model: got %q, want gemma4:12b", cur.Model)
	}
	if cur.Tokens == nil {
		t.Fatal("expected token counts from the SSE usage chunk")
	}
	if cur.Tokens.PromptEvalCount != 21 || cur.Tokens.EvalCount != 5 {
		t.Errorf("counts: got prompt=%d eval=%d, want 21/5", cur.Tokens.PromptEvalCount, cur.Tokens.EvalCount)
	}
	// OpenAI bodies carry no durations — latency must be derived from wall clock
	// (request observed -> completion) so the table's latency/tok-s/waterfall work.
	if cur.Tokens.LatencyMS <= 0 {
		t.Errorf("expected wall-clock latency > 0 for /v1, got %v", cur.Tokens.LatencyMS)
	}
	if cur.Tokens.PerSec <= 0 {
		t.Errorf("expected derived tok/s > 0 for /v1, got %v", cur.Tokens.PerSec)
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

// buildFakeOpenAIRequest produces a POST /v1/chat/completions request with an
// OpenAI-style body (model + messages).
func buildFakeOpenAIRequest() []byte {
	body := `{"model":"gemma4:12b","messages":[{"role":"user","content":"hi"}],"stream":true}`
	return []byte("POST /v1/chat/completions HTTP/1.1\r\nHost: 127.0.0.1:11434\r\nContent-Type: application/json\r\nContent-Length: " +
		itoa(len(body)) + "\r\n\r\n" + body)
}

// buildFakeOpenAIResponse produces a chunked text/event-stream response with a
// content chunk, a usage chunk (token counts), and the [DONE] sentinel —
// matching Ollama's real OpenAI-compatible streaming output.
func buildFakeOpenAIResponse() []byte {
	content := `data: {"object":"chat.completion.chunk","model":"gemma4:12b","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}` + "\n\n"
	usage := `data: {"object":"chat.completion.chunk","model":"gemma4:12b","choices":[],"usage":{"prompt_tokens":21,"completion_tokens":5,"total_tokens":26}}` + "\n\n"
	done := "data: [DONE]\n\n"

	chunk := func(s string) string { return itohex(len(s)) + "\r\n" + s + "\r\n" }
	return []byte("HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nTransfer-Encoding: chunked\r\n\r\n" +
		chunk(content) + chunk(usage) + chunk(done) + "0\r\n\r\n")
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
