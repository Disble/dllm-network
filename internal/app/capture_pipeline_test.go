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
		Window:        &fakeWindow{},
		Orchestrator:  &fakeOrchestrator{state: orchestrator.StateRunning},
		Config:        telemetry.Config{ShutdownTimeout: time.Second},
		CaptureSource: fake,
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

	snap, foundInference := waitForSnapshot(&emitted, 1500*time.Millisecond, func(snap dashboard.Snapshot) bool {
		return snap.Inference.Current.Status == inference.PhaseCompleted
	})
	if !foundInference {
		t.Fatalf("no emitted snapshot with PhaseCompleted inference within timeout; got %d snapshots", len(emitted))
	}
	if snap.Passive.Mode != "capture-active" {
		t.Errorf("expected passive mode 'capture-active', got %q", snap.Passive.Mode)
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

	done, found := waitForSnapshot(&emitted, 1500*time.Millisecond, func(snap dashboard.Snapshot) bool {
		return snap.Inference.Current.Status == inference.PhaseCompleted
	})
	if !found {
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

func waitForSnapshot(emitted *[]dashboard.Snapshot, timeout time.Duration, match func(dashboard.Snapshot) bool) (dashboard.Snapshot, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if snap, found := findSnapshot(*emitted, match); found {
			return snap, true
		}
		time.Sleep(20 * time.Millisecond)
	}

	return dashboard.Snapshot{}, false
}

func findSnapshot(emitted []dashboard.Snapshot, match func(dashboard.Snapshot) bool) (dashboard.Snapshot, bool) {
	for _, snap := range emitted {
		if match(snap) {
			return snap, true
		}
	}

	return dashboard.Snapshot{}, false
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

	inProgress, found := waitForSnapshot(&emitted, 1500*time.Millisecond, func(snap dashboard.Snapshot) bool {
		cur := snap.Inference.Current
		return cur.Endpoint == "/api/generate" && cur.Status == inference.PhaseInProgress
	})
	if !found {
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

	done, found := waitForSnapshot(&emitted, 1500*time.Millisecond, func(snap dashboard.Snapshot) bool {
		cur := snap.Inference.Current
		return cur.Endpoint == "/v1/chat/completions" && cur.Status == inference.PhaseCompleted
	})
	if !found {
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

// TestCapturePipeline_PopulatesGeneration asserts the pipeline derives the
// normalized Generation (assembled output text) on completion — for BOTH the
// Ollama-native NDJSON stream and the OpenAI SSE stream — so the Generation tab
// renders the model output without the frontend ever parsing a wire format.
func TestCapturePipeline_PopulatesGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("pipeline integration test skipped in short mode")
	}

	cases := []struct {
		name     string
		request  []byte
		response []byte
		endpoint string
	}{
		{"ollama_generate", buildFakeGenerateRequest(), buildFakeGenerateResponse(), "/api/generate"},
		{"openai_chat", buildFakeOpenAIRequest(), buildFakeOpenAIResponse(), "/v1/chat/completions"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseTime := time.Now()
			tuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 54321, DstPort: 11434}
			segments := []capture.Segment{
				{Tuple: tuple, Dir: capture.DirToServer, Payload: tc.request, SeqNo: 0, At: baseTime},
				{Tuple: tuple, Dir: capture.DirFromServer, Payload: tc.response, SeqNo: 0, At: baseTime.Add(time.Millisecond)},
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

			done, found := waitForSnapshot(&emitted, 1500*time.Millisecond, func(snap dashboard.Snapshot) bool {
				cur := snap.Inference.Current
				return cur.Endpoint == tc.endpoint && cur.Status == inference.PhaseCompleted
			})
			if !found {
				t.Fatalf("no completed %s inference; got %d snapshots", tc.endpoint, len(emitted))
			}

			gen := done.Inference.Current.Generation
			if gen == nil {
				t.Fatal("expected Generation to be populated on completion")
			}
			if gen.Output != "hi" {
				t.Errorf("Generation.Output: got %q, want %q", gen.Output, "hi")
			}
		})
	}
}

// TestCapturePipeline_LiveGenerationGrowsDuringStreaming proves the pipeline
// publishes normalized Generation snapshots during streaming, not only after
// completion, for both Ollama NDJSON and OpenAI SSE endpoints.
func TestCapturePipeline_LiveGenerationGrowsDuringStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("pipeline integration test skipped in short mode")
	}

	testCases := []struct {
		name          string
		endpoint      string
		request       []byte
		responseParts [][]byte
		wantGrowth    []string
	}{
		{
			name:          "ollama_generate",
			endpoint:      "/api/generate",
			request:       buildFakeGenerateRequest(),
			responseParts: buildFakeGenerateStreamingResponseParts("h", "i"),
			wantGrowth:    []string{"h", "hi"},
		},
		{
			name:          "openai_chat",
			endpoint:      "/v1/chat/completions",
			request:       buildFakeOpenAIRequest(),
			responseParts: buildFakeOpenAIStreamingResponseParts("h", "i"),
			wantGrowth:    []string{"h", "hi"},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			baseTime := time.Now()
			tuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 54321, DstPort: 11434}
			segments := []capture.Segment{{Tuple: tuple, Dir: capture.DirToServer, Payload: tt.request, SeqNo: 0, At: baseTime}}

			seqNo := uint32(0)
			for i, part := range tt.responseParts {
				segments = append(segments, capture.Segment{
					Tuple:   tuple,
					Dir:     capture.DirFromServer,
					Payload: part,
					SeqNo:   seqNo,
					At:      baseTime.Add(time.Duration(i+1) * 10 * time.Millisecond),
				})
				seqNo += uint32(len(part))
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

			done, found := waitForSnapshot(&emitted, 1500*time.Millisecond, func(snap dashboard.Snapshot) bool {
				cur := snap.Inference.Current
				return cur.Endpoint == tt.endpoint && cur.Status == inference.PhaseCompleted
			})
			if !found {
				t.Fatalf("no completed %s inference; got %d snapshots", tt.endpoint, len(emitted))
			}

			growth := streamedGenerationOutputs(emitted, tt.endpoint)
			if !containsOrderedOutputs(growth, tt.wantGrowth) {
				t.Fatalf("in-progress generation growth = %v, want ordered subsequence %v", growth, tt.wantGrowth)
			}

			completed := done.Inference.Current.Generation
			if completed == nil {
				t.Fatal("expected completed generation snapshot")
			}
			wantFinal := tt.wantGrowth[len(tt.wantGrowth)-1]
			if completed.Output != wantFinal {
				t.Fatalf("completed Generation.Output = %q, want %q", completed.Output, wantFinal)
			}
			if last := lastOutput(growth); last != wantFinal {
				t.Fatalf("final in-progress output = %q, want parity with completed %q", last, wantFinal)
			}
		})
	}
}

// TestCapturePipeline_GenerationResetAcrossKeepAliveReuse proves a new request
// on the same keep-alive tuple starts from a fresh generation state.
func TestCapturePipeline_GenerationResetAcrossKeepAliveReuse(t *testing.T) {
	if testing.Short() {
		t.Skip("pipeline integration test skipped in short mode")
	}

	baseTime := time.Now()
	tuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 54321, DstPort: 11434}
	firstRequest := buildFakeGenerateRequestWithPrompt("first")
	firstParts := buildFakeGenerateStreamingResponseParts("h", "i")
	secondRequest := buildFakeGenerateRequestWithPrompt("second")
	secondParts := buildFakeGenerateStreamingResponseParts("o", "k")

	segments := []capture.Segment{
		{Tuple: tuple, Dir: capture.DirToServer, Payload: firstRequest, SeqNo: 0, At: baseTime},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: firstParts[0], SeqNo: 0, At: baseTime.Add(10 * time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: firstParts[1], SeqNo: uint32(len(firstParts[0])), At: baseTime.Add(20 * time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: firstParts[2], SeqNo: uint32(len(firstParts[0]) + len(firstParts[1])), At: baseTime.Add(30 * time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: firstParts[3], SeqNo: uint32(len(firstParts[0]) + len(firstParts[1]) + len(firstParts[2])), At: baseTime.Add(40 * time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirToServer, Payload: secondRequest, SeqNo: uint32(len(firstRequest)), At: baseTime.Add(50 * time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: secondParts[0], SeqNo: uint32(totalLen(firstParts...)), At: baseTime.Add(60 * time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: secondParts[1], SeqNo: uint32(totalLen(firstParts...) + len(secondParts[0])), At: baseTime.Add(70 * time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: secondParts[2], SeqNo: uint32(totalLen(firstParts...) + len(secondParts[0]) + len(secondParts[1])), At: baseTime.Add(80 * time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: secondParts[3], SeqNo: uint32(totalLen(firstParts...) + len(secondParts[0]) + len(secondParts[1]) + len(secondParts[2])), At: baseTime.Add(90 * time.Millisecond)},
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

	secondDone, found := waitForSnapshot(&emitted, 1500*time.Millisecond, func(snap dashboard.Snapshot) bool {
		cur := snap.Inference.Current
		return cur.Endpoint == "/api/generate" && cur.Status == inference.PhaseCompleted && cur.Generation != nil && cur.Generation.Output == "ok"
	})
	if !found {
		t.Fatalf("no completed second keep-alive inference; got %d snapshots", len(emitted))
	}

	if secondDone.Inference.Current.Generation == nil {
		t.Fatal("expected completed generation for second keep-alive request")
	}
	if got := secondDone.Inference.Current.Generation.Output; got != "ok" {
		t.Fatalf("second keep-alive Generation.Output = %q, want %q", got, "ok")
	}
}

// TestCapturePipeline_GenerationCleanupOnIdleCancellation proves stale partial
// generation state is evicted on idle cancellation before the tuple is reused
// by a later exchange. The fresh exchange must start from a clean accumulator,
// so its final output cannot contain bytes from the stale partial stream.
func TestCapturePipeline_GenerationCleanupOnIdleCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("pipeline integration test skipped in short mode")
	}

	baseTime := time.Now()
	tuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 54321, DstPort: 11434}
	otherTuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 54322, DstPort: 11434}
	staleRequest := buildFakeGenerateRequestWithPrompt("stale")
	freshRequest := buildFakeGenerateRequestWithPrompt("fresh")
	sweepRequest := buildFakeGenerateRequestWithPrompt("sweep")

	firstParts := buildFakeGenerateStreamingResponseParts("h", "i")
	secondParts := buildFakeGenerateStreamingResponseParts("o", "k")

	segments := []capture.Segment{
		{Tuple: tuple, Dir: capture.DirToServer, Payload: staleRequest, SeqNo: 0, At: baseTime},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: firstParts[0], SeqNo: 0, At: baseTime.Add(10 * time.Millisecond)},
		{Tuple: otherTuple, Dir: capture.DirToServer, Payload: sweepRequest, SeqNo: 0, At: baseTime.Add(captureIdleTimeout + captureSweepInterval)},
		{Tuple: tuple, Dir: capture.DirToServer, Payload: freshRequest, SeqNo: uint32(len(staleRequest)), At: baseTime.Add(captureIdleTimeout + captureSweepInterval + 10*time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: secondParts[0], SeqNo: uint32(len(firstParts[0])), At: baseTime.Add(captureIdleTimeout + captureSweepInterval + 20*time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: secondParts[1], SeqNo: uint32(len(firstParts[0]) + len(secondParts[0])), At: baseTime.Add(captureIdleTimeout + captureSweepInterval + 30*time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: secondParts[2], SeqNo: uint32(len(firstParts[0]) + len(secondParts[0]) + len(secondParts[1])), At: baseTime.Add(captureIdleTimeout + captureSweepInterval + 40*time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: secondParts[3], SeqNo: uint32(len(firstParts[0]) + len(secondParts[0]) + len(secondParts[1]) + len(secondParts[2])), At: baseTime.Add(captureIdleTimeout + captureSweepInterval + 50*time.Millisecond)},
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

	freshDone, found := waitForSnapshot(&emitted, 1500*time.Millisecond, func(snap dashboard.Snapshot) bool {
		cur := snap.Inference.Current
		return cur.Endpoint == "/api/generate" && cur.Status == inference.PhaseCompleted && cur.Generation != nil && cur.Generation.Output == "ok"
	})
	if !found {
		t.Fatalf("no completed fresh inference after idle eviction; got %d snapshots", len(emitted))
	}

	if freshDone.Inference.Current.Generation == nil {
		t.Fatal("expected completed generation for fresh exchange")
	}
	if got := freshDone.Inference.Current.Generation.Output; got != "ok" {
		t.Fatalf("fresh Generation.Output after idle eviction = %q, want %q", got, "ok")
	}
}

// TestCapturePipeline_OpenAITTFTSplitsTiming asserts that for an OpenAI stream
// (no server-side durations) the pipeline derives REAL waterfall phases from the
// observed packet times: LoadDuration = time-to-first-token (request -> first
// content chunk), EvalDuration = generation span (first chunk -> completion).
// The two phases must sum to the total so the waterfall renders meaningful
// segments instead of one flat "other" block.
func TestCapturePipeline_OpenAITTFTSplitsTiming(t *testing.T) {
	if testing.Short() {
		t.Skip("pipeline integration test skipped in short mode")
	}

	header := "HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nTransfer-Encoding: chunked\r\n\r\n"
	contentSSE := `data: {"object":"chat.completion.chunk","model":"gemma4:12b","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}` + "\n\n"
	usageSSE := `data: {"object":"chat.completion.chunk","model":"gemma4:12b","choices":[],"usage":{"prompt_tokens":21,"completion_tokens":5,"total_tokens":26}}` + "\n\n"
	doneSSE := "data: [DONE]\n\n"
	chunk := func(s string) string { return itohex(len(s)) + "\r\n" + s + "\r\n" }

	// Split the response so the first content chunk and the [DONE] arrive in
	// SEPARATE segments at different observed times.
	partA := []byte(header + chunk(contentSSE))
	partB := []byte(chunk(usageSSE) + chunk(doneSSE) + "0\r\n\r\n")

	baseTime := time.Now()
	tuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 54321, DstPort: 11434}
	segments := []capture.Segment{
		{Tuple: tuple, Dir: capture.DirToServer, Payload: buildFakeOpenAIRequest(), SeqNo: 0, At: baseTime},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: partA, SeqNo: 0, At: baseTime.Add(10 * time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: partB, SeqNo: uint32(len(partA)), At: baseTime.Add(50 * time.Millisecond)},
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

	done, found := waitForSnapshot(&emitted, 1500*time.Millisecond, func(snap dashboard.Snapshot) bool {
		cur := snap.Inference.Current
		return cur.Endpoint == "/v1/chat/completions" && cur.Status == inference.PhaseCompleted
	})
	if !found {
		t.Fatalf("no completed /v1/chat/completions inference; got %d snapshots", len(emitted))
	}

	tok := done.Inference.Current.Tokens
	if tok == nil {
		t.Fatal("expected token stats with derived timing")
	}
	if tok.LoadDuration != 10*time.Millisecond {
		t.Errorf("LoadDuration (TTFT): got %v, want 10ms", tok.LoadDuration)
	}
	if tok.EvalDuration != 40*time.Millisecond {
		t.Errorf("EvalDuration (generation span): got %v, want 40ms", tok.EvalDuration)
	}
	if tok.TotalDuration != 50*time.Millisecond {
		t.Errorf("TotalDuration: got %v, want 50ms", tok.TotalDuration)
	}
	if tok.LoadDuration+tok.EvalDuration != tok.TotalDuration {
		t.Errorf("phases must sum to total: load=%v eval=%v total=%v", tok.LoadDuration, tok.EvalDuration, tok.TotalDuration)
	}
}

// TestCapturePipeline_InProgressAtStaysStableAcrossChunks guards the flicker fix:
// the in-progress row's At is the START time the frontend measures live elapsed
// against (now - At). The extractor stamps At=time.Now() on EVERY call, so for a
// stream the pipeline MUST pin At to the request observation time — otherwise each
// streamed chunk resets the elapsed to ~0 and the latency/waterfall cells flicker
// between a value and the "unavailable" em-dash. Every in-progress snapshot for
// the request must report the request's observed time, regardless of when each
// chunk arrived.
func TestCapturePipeline_InProgressAtStaysStableAcrossChunks(t *testing.T) {
	if testing.Short() {
		t.Skip("pipeline integration test skipped in short mode")
	}

	header := "HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nTransfer-Encoding: chunked\r\n\r\n"
	contentSSE := `data: {"object":"chat.completion.chunk","model":"gemma4:12b","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}` + "\n\n"
	doneSSE := "data: [DONE]\n\n"
	chunk := func(s string) string { return itohex(len(s)) + "\r\n" + s + "\r\n" }

	// In-progress content chunk and the terminal [DONE] arrive in SEPARATE
	// segments, observed well AFTER the request.
	partA := []byte(header + chunk(contentSSE))
	partB := []byte(chunk(doneSSE) + "0\r\n\r\n")

	baseTime := time.Now()
	tuple := capture.FourTuple{SrcIP: "127.0.0.1", DstIP: "127.0.0.1", SrcPort: 54321, DstPort: 11434}
	segments := []capture.Segment{
		{Tuple: tuple, Dir: capture.DirToServer, Payload: buildFakeOpenAIRequest(), SeqNo: 0, At: baseTime},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: partA, SeqNo: 0, At: baseTime.Add(30 * time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirFromServer, Payload: partB, SeqNo: uint32(len(partA)), At: baseTime.Add(60 * time.Millisecond)},
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

	// Wait until the exchange completes so every in-progress emission has happened.
	if _, found := waitForSnapshot(&emitted, 1500*time.Millisecond, func(snap dashboard.Snapshot) bool {
		cur := snap.Inference.Current
		return cur.Endpoint == "/v1/chat/completions" && cur.Status == inference.PhaseCompleted
	}); !found {
		t.Fatalf("no completed /v1/chat/completions inference; got %d snapshots", len(emitted))
	}

	sawInProgress := false
	for _, snap := range emitted {
		cur := snap.Inference.Current
		if cur.Endpoint != "/v1/chat/completions" || cur.Status != inference.PhaseInProgress {
			continue
		}
		sawInProgress = true
		if !cur.At.Equal(baseTime) {
			t.Errorf("in-progress At must equal the request observation time %v (stable across chunks); got %v", baseTime, cur.At)
		}
	}
	if !sawInProgress {
		t.Fatal("expected at least one in-progress snapshot for /v1/chat/completions")
	}
}

// TestInferenceID_UniqueAcrossRuns guards against the stale-detail bug: ids must
// be unique ACROSS process runs, because the durable SQLite store persists them
// and App.InferenceDetail looks records up by id. A per-run counter alone (seq
// resetting to 0 each launch) made a new run's "inf-1" collide with a previous
// run's persisted "inf-1", so the detail panel showed a ghost record from an
// earlier session. Embedding a per-run nonce eliminates the collision.
func TestInferenceID_UniqueAcrossRuns(t *testing.T) {
	t.Parallel()

	run1, run2 := newRunID(), newRunID()
	if run1 == run2 {
		t.Fatalf("expected distinct run ids, got %q twice", run1)
	}
	if inferenceID(run1, 1) == inferenceID(run2, 1) {
		t.Errorf("same seq from different runs must NOT collide: %q", inferenceID(run1, 1))
	}
	if stable, again := inferenceID(run1, 1), inferenceID(run1, 1); stable != again {
		t.Errorf("same run+seq must be stable: %q vs %q", stable, again)
	}
	if inferenceID(run1, 1) == inferenceID(run1, 2) {
		t.Error("different seq in the same run must differ")
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

func (t *trackingCaptureSource) Open() error { return t.inner.Open() }
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

func buildFakeGenerateRequestWithPrompt(prompt string) []byte {
	body := `{"model":"llama3","prompt":"` + prompt + `"}`
	return []byte("POST /api/generate HTTP/1.1\r\nHost: 127.0.0.1:11434\r\nContent-Type: application/json\r\nContent-Length: " +
		itoa(len(body)) + "\r\n\r\n" + body)
}

func buildFakeGenerateStreamingResponseParts(tokens ...string) [][]byte {
	parts := [][]byte{[]byte("HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\nContent-Type: application/x-ndjson\r\n\r\n")}
	for _, token := range tokens {
		line := `{"model":"llama3","response":"` + token + `","done":false}` + "\n"
		parts = append(parts, []byte(itohex(len(line))+"\r\n"+line+"\r\n"))
	}
	terminal := `{"model":"llama3","response":"","done":true,"prompt_eval_count":5,"eval_count":2,"eval_duration":200000000,"total_duration":250000000,"load_duration":10000000}` + "\n"
	parts = append(parts, []byte(itohex(len(terminal))+"\r\n"+terminal+"\r\n0\r\n\r\n"))
	return parts
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

func buildFakeOpenAIStreamingResponseParts(tokens ...string) [][]byte {
	parts := [][]byte{[]byte("HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nTransfer-Encoding: chunked\r\n\r\n")}
	for _, token := range tokens {
		line := `data: {"object":"chat.completion.chunk","model":"gemma4:12b","choices":[{"index":0,"delta":{"content":"` + token + `"},"finish_reason":null}]}` + "\n\n"
		parts = append(parts, []byte(itohex(len(line))+"\r\n"+line+"\r\n"))
	}
	done := "data: [DONE]\n\n"
	parts = append(parts, []byte(itohex(len(done))+"\r\n"+done+"\r\n0\r\n\r\n"))
	return parts
}

func streamedGenerationOutputs(emitted []dashboard.Snapshot, endpoint string) []string {
	outputs := make([]string, 0)
	for _, snap := range emitted {
		cur := snap.Inference.Current
		if cur.Endpoint != endpoint || cur.Status != inference.PhaseInProgress || cur.Generation == nil {
			continue
		}
		if len(outputs) == 0 || outputs[len(outputs)-1] != cur.Generation.Output {
			outputs = append(outputs, cur.Generation.Output)
		}
	}
	return outputs
}

func containsOrderedOutputs(got, want []string) bool {
	if len(want) == 0 {
		return true
	}
	idx := 0
	for _, output := range got {
		if output == want[idx] {
			idx++
			if idx == len(want) {
				return true
			}
		}
	}
	return false
}

func lastOutput(outputs []string) string {
	if len(outputs) == 0 {
		return ""
	}
	return outputs[len(outputs)-1]
}

func totalLen(parts ...[]byte) int {
	total := 0
	for _, part := range parts {
		total += len(part)
	}
	return total
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
