package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"dllm-network/internal/capture"
	"dllm-network/internal/capture/httpx"
	"dllm-network/internal/capture/reassembly"
	"dllm-network/internal/dashboard"
	"dllm-network/internal/events"
	"dllm-network/internal/store"
	"dllm-network/internal/telemetry/inference"
)

// topicInferenceCompleted is the events.Bus topic the capture pipeline
// publishes to on each completed inference, durably persisted by
// internal/persistence.Subscriber (design D7 write trigger). Kept as its
// own constant here (matching internal/persistence's own copy) so this
// package does not need to import internal/persistence just for the topic
// name — the two packages are coupled only through the topic string.
const topicInferenceCompleted = "inference.completed"

const (
	// captureIdleTimeout is how long a connection may be silent before its
	// per-connection bookkeeping is evicted. Matches the reassembler's default
	// idle timeout so the two layers drop a dead connection together.
	captureIdleTimeout = 30 * time.Second
	// captureSweepInterval bounds how often the recv-loop runs the idle sweep.
	// It is measured against OBSERVED segment timestamps (not wall-clock), so
	// the sweep cost stays negligible relative to per-segment work and needs no
	// separate timer goroutine racing the lock-free per-connection maps.
	captureSweepInterval = 5 * time.Second
)

// isTerminalRecvErr returns true for errors that should cause the recv-loop to
// exit cleanly: context cancellation, deadline exceeded, or the fake source's
// exhausted sentinel.
func isTerminalRecvErr(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, capture.ErrFakeExhausted)
}

// connParsers holds the request (ToServer) and response (FromServer) HTTP
// parsers for a single connection keyed by its canonical FourTuple.
type connParsers struct {
	req  *httpx.Parser
	resp *httpx.Parser
}

// captureState holds all per-connection bookkeeping for the recv-loop. Maps are
// keyed by canonical FourTuple so the request and response for one logical TCP
// connection share a row regardless of OS-reported src/dst swapping.
type captureState struct {
	reassembler    *reassembly.Reassembler
	parsers        map[reassembly.FourTuple]*connParsers
	requestBuf     map[reassembly.FourTuple]httpx.Message
	reqID          map[reassembly.FourTuple]string
	respAccum      map[reassembly.FourTuple][]byte
	genAccum       map[reassembly.FourTuple]*inference.GenerationAccumulator
	reqTime        map[reassembly.FourTuple]time.Time
	firstTokenTime map[reassembly.FourTuple]time.Time
	lastSeen       map[reassembly.FourTuple]time.Time
	lastSweep      time.Time
	runID          string
	idSeq          int
	current        inference.Inference
	segCount       int
}

// newCaptureState initialises the per-connection bookkeeping for one recv-loop
// run. Maps are pre-allocated so the first recv does not force an allocation on
// the hot path.
func newCaptureState() *captureState {
	return &captureState{
		reassembler:    reassembly.New(),
		parsers:        make(map[reassembly.FourTuple]*connParsers),
		requestBuf:     make(map[reassembly.FourTuple]httpx.Message),
		reqID:          make(map[reassembly.FourTuple]string),
		respAccum:      make(map[reassembly.FourTuple][]byte),
		genAccum:       make(map[reassembly.FourTuple]*inference.GenerationAccumulator),
		reqTime:        make(map[reassembly.FourTuple]time.Time),
		firstTokenTime: make(map[reassembly.FourTuple]time.Time),
		lastSeen:       make(map[reassembly.FourTuple]time.Time),
		runID:          newRunID(),
	}
}

// ensureParsers returns the connParsers for key, creating it with fresh HTTP
// parsers if this is the first segment for this connection.
func (s *captureState) ensureParsers(key reassembly.FourTuple) *connParsers {
	cp, ok := s.parsers[key]
	if !ok {
		cp = &connParsers{req: httpx.NewParser(), resp: httpx.NewParser()}
		s.parsers[key] = cp
	}
	return cp
}

// evictIdle drops bookkeeping for connections silent past captureIdleTimeout —
// bounding memory for connections whose FIN was never observed (dropped
// packets, ungraceful client exit). A connection whose request never completed
// is surfaced as a PhaseCancelled inference, with its observed time span
// preserved for the waterfall. Cancellations go to the recent ring (a full
// list in every snapshot), NOT through current, so a sweep that cancels
// several connections is not collapsed by the snapshot coalescer.
func (s *captureState) evictIdle(now time.Time, p *inferencePipeline) {
	for key, seen := range s.lastSeen {
		if now.Sub(seen) < captureIdleTimeout {
			continue
		}
		// reqTime[key] is set on the request and deleted on completion, so its
		// presence means this connection's request never completed.
		if reqAt, pending := s.reqTime[key]; pending {
			if req, ok := s.requestBuf[key]; ok {
				if cancelled, ok := buildCancelledInference(p.extractor, req, s.reqID[key], reqAt, seen); ok {
					p.recent.RecordInferenceCancellation(cancelled)
					if p.bus != nil {
						p.bus.Publish(events.Event{Topic: topicInferenceCompleted, Payload: cancelled})
					}
				}
			}
		}
		delete(s.parsers, key)
		delete(s.requestBuf, key)
		delete(s.reqID, key)
		delete(s.respAccum, key)
		delete(s.genAccum, key)
		delete(s.reqTime, key)
		delete(s.firstTokenTime, key)
		delete(s.lastSeen, key)
	}
	s.reassembler.EvictIdle(now)
}

// inferencePipeline composes:
//
//	CaptureSource → reassembly.Reassembler → httpx.Parser (per direction) →
//	inference.Extractor → store.Recent inference events → dashboard.Publisher
//
// It is deliberately driver-free: the CaptureSource is injected as an
// interface so unit tests can supply a fake source (capture.NewFakeSource)
// without a WinDivert driver or administrator privileges.
//
// One inferencePipeline is created per App and lives for the app's lifetime.
// It is started from Startup and stopped from Quit.
type inferencePipeline struct {
	src       capture.CaptureSource
	recent    *store.Recent
	publisher snapshotProjector
	extractor *inference.Extractor
	// bus is the optional events.Bus the pipeline publishes completed
	// inferences to for durable persistence (internal/persistence.Subscriber
	// subscribes to topicInferenceCompleted). Nil-safe: when nil (e.g. tests
	// that only exercise the live dashboard projection), publish is skipped.
	bus *events.Bus

	// cancelCapture stops the capture goroutine's context.
	cancelCapture context.CancelFunc
	done          chan struct{}
}

// newInferencePipeline creates a pipeline wired to the given source, store,
// and publisher. It does NOT start the goroutine — call run(). bus may be
// nil; see the inferencePipeline.bus field doc.
func newInferencePipeline(
	src capture.CaptureSource,
	recent *store.Recent,
	publisher snapshotProjector,
	bus *events.Bus,
) *inferencePipeline {
	return &inferencePipeline{
		src:       src,
		recent:    recent,
		publisher: publisher,
		extractor: inference.NewExtractor(),
		bus:       bus,
		done:      make(chan struct{}),
	}
}

// run opens the capture source and starts the Recv-loop goroutine. The loop
// runs until ctx is cancelled (from Quit) or the source is exhausted.
//
// Graceful degradation: if the source is not Active after Open (unelevated or
// noop), the function still starts the goroutine but Recv immediately returns
// ctx.Err() (noop) or ErrFakeExhausted (fake), so the goroutine exits cleanly
// without any panic or deadlock.
func (p *inferencePipeline) run(ctx context.Context) {
	captureCtx, cancel := context.WithCancel(ctx)
	p.cancelCapture = cancel

	_ = p.src.Open() // errors tolerated — Status() will reflect degradation

	go p.recvLoop(captureCtx)
}

// stop cancels the capture goroutine and waits for it to exit.
func (p *inferencePipeline) stop() {
	if p.cancelCapture != nil {
		p.cancelCapture()
	}
	<-p.done
	_ = p.src.Close()
}

// recvLoop is the long-running goroutine that pulls segments from the source,
// reassembles them per connection, parses HTTP messages, extracts inference
// metrics, and emits updated snapshots via the publisher.
func (p *inferencePipeline) recvLoop(ctx context.Context) {
	defer close(p.done)

	s := newCaptureState()

	status := p.src.Status()
	capLog("recvLoop start: active=%v elevated=%v reason=%q", status.Active, status.Elevated, status.Reason)

	for {
		seg, err := p.src.Recv(ctx)
		if err != nil {
			if isTerminalRecvErr(err) {
				capLog("recvLoop exit: %v (total segments=%d)", err, s.segCount)
				return
			}
			capLog("recv non-fatal error: %v", err)
			continue
		}
		s.segCount++
		if len(seg.Payload) > 0 {
			capLog("seg #%d dir=%d len=%d src=%s:%d dst=%s:%d seq=%d", s.segCount, seg.Dir, len(seg.Payload), seg.Tuple.SrcIP, seg.Tuple.SrcPort, seg.Tuple.DstIP, seg.Tuple.DstPort, seg.SeqNo)
		}

		// Refresh status after each recv (source may have transitioned).
		status = p.src.Status()

		// Feed segment into reassembler. capture.Segment has identical fields to
		// reassembly.Segment (FourTuple and Direction are type-aliases from the
		// reassembly package), so the conversion is field-by-field.
		streams := s.reassembler.Push(reassembly.Segment{
			Tuple:   seg.Tuple,
			Dir:     seg.Dir,
			Payload: seg.Payload,
			SeqNo:   seg.SeqNo,
			At:      seg.At,
		})

		for _, stream := range streams {
			// Key per logical connection, not per packet direction: the OS
			// reports the request (client->server) and response (server->client)
			// with SWAPPED 4-tuples, so we canonicalise to pair them.
			key := canonicalTuple(stream.Tuple)
			s.lastSeen[key] = seg.At

			cp := s.ensureParsers(key)

			switch stream.Dir {
			case capture.DirToServer:
				msgs := cp.req.Feed(stream.Payload)
				p.processRequestStream(key, msgs, seg.At, s)
			case capture.DirFromServer:
				msgs := cp.resp.Feed(stream.Payload)
				p.processResponseStream(key, msgs, seg.At, s)
			}
		}

		// Periodically evict idle connections (bounding state) and surface any
		// stuck in-progress request as cancelled. Driven by observed segment
		// time so it costs nothing between sweeps.
		if s.lastSweep.IsZero() {
			s.lastSweep = seg.At
		} else if seg.At.Sub(s.lastSweep) >= captureSweepInterval {
			s.evictIdle(seg.At, p)
			s.lastSweep = seg.At
		}

		// Build CaptureInput from live source status.

		captureInput := buildCaptureInput(status, s.current)

		_, _ = p.publisher.Publish(ctx, dashboard.ProjectionInput{
			Capture: captureInput,
			Inference: dashboard.InferenceState{
				Current: s.current,
				Recent:  p.recent.InferenceEvents(),
			},
		})
	}
}

// processRequestStream handles parsed request messages for a connection.
// It buffers the request, assigns a stable inference id, resets per-exchange
// state for keep-alive reuse, and emits an in-progress inference for the
// live dashboard.
func (p *inferencePipeline) processRequestStream(key reassembly.FourTuple, msgs []httpx.Message, at time.Time, s *captureState) {
	for _, m := range msgs {
		capLog("  req msg: kind=%d method=%s path=%s", m.Kind, m.Method, m.Path)
		if m.Kind != httpx.KindRequest {
			continue
		}
		s.requestBuf[key] = m
		s.idSeq++
		s.reqID[key] = inferenceID(s.runID, s.idSeq)
		s.reqTime[key] = at        // for wall-clock latency (OpenAI /v1)
		delete(s.respAccum, key)   // fresh exchange on this connection
		delete(s.firstTokenTime, key) // reset TTFT for keep-alive reuse
		delete(s.genAccum, key)
		s.genAccum[key] = inference.NewGenerationAccumulator(m.Path)

		// Emit an in-progress inference the moment the request is observed,
		// before any response byte arrives. For stream:false generations the
		// server stays silent until done, so a response-only trigger would leave
		// the row invisible for seconds. Pairing the request with an empty
		// response yields PhaseInProgress (Tokens nil — never fabricated).
		// Metadata-only polls (/api/tags, /api/ps, …) are skipped so they don't
		// overwrite the displayed inference.
		if inProgress, ok := p.extractor.FromExchange(m, httpx.Message{}); ok && inProgress.Status != inference.PhaseMetadataOnly {
			inProgress.ID = s.reqID[key]
			// Stamp the request observation time, not the extractor's per-call
			// time.Now(): the in-progress row's At is the START the frontend
			// measures live elapsed against. It MUST stay stable for the
			// request's lifetime (see the response path).
			inProgress.At = s.reqTime[key]
			s.current = inProgress
		}
	}
}

// processResponseStream handles parsed response messages for a connection.
// It accumulates the assembled body, tracks time-to-first-token, recovers
// token counts from streamed completions, publishes completed inferences to
// the recent ring and the durable persistence bus, and releases per-exchange
// state when an exchange finishes.
func (p *inferencePipeline) processResponseStream(key reassembly.FourTuple, msgs []httpx.Message, at time.Time, s *captureState) {
	for _, m := range msgs {
		capLog("  resp msg: kind=%d status=%d done=%v bodyLen=%d", m.Kind, m.StatusCode, m.Done, len(m.Body))
		if m.Kind != httpx.KindResponse {
			continue
		}
		req, hasReq := s.requestBuf[key]
		if !hasReq {
			capLog("    no buffered request for key %s:%d/%s:%d", key.SrcIP, key.SrcPort, key.DstIP, key.DstPort)
			continue
		}
		// Accumulate the raw response line so the detail inspector can show
		// the assembled (streamed) response body.
		acc := append(s.respAccum[key], m.Body...)
		acc = append(acc, '\n')
		s.respAccum[key] = acc

		if s.genAccum[key] == nil {
			s.genAccum[key] = inference.NewGenerationAccumulator(req.Path)
		}
		s.genAccum[key].Feed(m.Body)

		// Observe the time-to-first-token: the first chunk carrying generated
		// content marks the prompt->generation transition, used to split the
		// waterfall for streams without server-side durations.
		if s.firstTokenTime[key].IsZero() && inference.HasGeneratedContent(req.Path, m.Body) {
			s.firstTokenTime[key] = at
		}

		inf, ok := p.extractor.FromExchange(req, m)
		capLog("    FromExchange ok=%v status=%d model=%s hasTokens=%v", ok, inf.Status, inf.Model, inf.Tokens != nil)
		if !ok {
			continue
		}
		// Ignore non-inference exchanges (e.g. /api/tags, /api/ps, /api/version
		// polls — including this app's own orchestrator polling). They must not
		// overwrite the displayed inference.
		if inf.Status == inference.PhaseMetadataOnly {
			continue
		}
		// Stable id (shared across in-progress/completed) + assembled response
		// body override the per-line extractor values.
		inf.ID = s.reqID[key]
		// Pin the in-progress row's At to the request observation time. The
		// extractor stamps At=time.Now() on EVERY chunk, so without this the
		// frontend's live elapsed (now - At) resets to ~0 on each streamed
		// chunk — making the latency and waterfall cells flicker between a
		// value and the "unavailable" em-dash. Completed rows keep the
		// extractor's completion timestamp.
		if inf.Status != inference.PhaseCompleted {
			inf.At = s.reqTime[key]
		}
		inf.ResponseBody, inf.ResponseBodyTruncated = inference.TruncateBody(s.respAccum[key])
		if gen := s.genAccum[key]; gen != nil {
			inf.Generation = gen.Build()
		}
		// OpenAI streaming completes on the [DONE] sentinel, which carries no
		// counts — the `usage` arrived in an earlier SSE chunk. Recover the
		// counts from the assembled body when the per-line extractor could not
		// (Tokens still nil on a completed exchange).
		if inf.Status == inference.PhaseCompleted && inf.Tokens == nil {
			inf.Tokens = inference.ExtractOpenAIStats(s.respAccum[key])
			if inf.Tokens == nil {
				// No usage chunk (include_usage not set): still surface the
				// observed wall-clock timing so latency and the waterfall render.
				inf.Tokens = &inference.TokenStats{}
			}
			deriveStreamingTiming(inf.Tokens, s.reqTime[key], s.firstTokenTime[key], at)
		}
		s.current = inf
		if inf.Status == inference.PhaseCompleted {
			p.recent.RecordInferenceCompletion(inf)
			// Sibling durable-write trigger (design D7): the ring above serves
			// the live dashboard projection; this publish feeds
			// internal/persistence.Subscriber for cross-process durable storage.
			// Bus.Publish is synchronous but the subscriber's handler only does
			// a non-blocking channel send, so this never stalls the capture
			// loop.
			if p.bus != nil {
				p.bus.Publish(events.Event{Topic: topicInferenceCompleted, Payload: inf})
			}
			delete(s.respAccum, key) // exchange done; release buffer
			delete(s.genAccum, key)
			delete(s.reqTime, key)
			delete(s.firstTokenTime, key)
		}
	}
}

// buildCancelledInference derives a terminal "cancelled" inference for a
// connection whose request was observed but never completed before the
// connection went idle past the capture timeout. It reuses the extractor to
// recover request-side metadata (model, endpoint, headers, body) then marks the
// result PhaseCancelled and attaches the observed wall-clock span (request →
// last activity) so the waterfall can still render the bar. Returns ok=false
// for metadata-only requests (polls), which must not surface as cancelled rows.
func buildCancelledInference(extractor *inference.Extractor, req httpx.Message, id string, reqAt, lastAt time.Time) (inference.Inference, bool) {
	inf, ok := extractor.FromExchange(req, httpx.Message{})
	if !ok || inf.Status == inference.PhaseMetadataOnly {
		return inference.Inference{}, false
	}
	inf.ID = id
	inf.At = reqAt
	inf.Status = inference.PhaseCancelled
	inf.Tokens = cancelledTiming(reqAt, lastAt)
	return inf, true
}

// cancelledTiming returns the observed wall-clock span as a TokenStats carrying
// ONLY duration/latency — eval/prompt counts and tokens/sec stay zero because
// no tokens were ever measured. The span feeds the waterfall (TotalDuration →
// totalMS); the absent counts stay honestly zero rather than fabricated.
// Returns nil when the span is non-positive (same-instant edge case).
func cancelledTiming(reqAt, lastAt time.Time) *inference.TokenStats {
	elapsed := lastAt.Sub(reqAt)
	if elapsed <= 0 {
		return nil
	}
	return &inference.TokenStats{
		TotalDuration: elapsed,
		LatencyMS:     float64(elapsed) / 1e6,
	}
}

// deriveStreamingTiming fills the waterfall phases + latency + tokens/sec for a
// streamed exchange whose body carries no server-side durations (OpenAI /v1).
// It is a best-effort PASSIVE measurement from observed packet times — not the
// model's internal eval_duration:
//
//	LoadDuration  = time-to-first-token   (request observed -> first content chunk)
//	EvalDuration  = generation span       (first content chunk -> completion)
//	TotalDuration = full wall-clock span  (request observed -> completion)
//
// When the first-token time was never observed (no content chunk seen, or it
// arrived in the same packet as completion) the whole span is attributed to
// generation and the phase split is left zero, so the bar still renders without
// a fabricated TTFT. tokens/sec is computed against the GENERATION span so it
// reflects decode speed, not queue+prompt latency. No-op when stats or the
// request time are unavailable, keeping counts-only completions honest.
func deriveStreamingTiming(stats *inference.TokenStats, reqAt, firstTokenAt, doneAt time.Time) {
	if stats == nil || reqAt.IsZero() {
		return
	}
	total := doneAt.Sub(reqAt)
	if total <= 0 {
		return
	}
	stats.TotalDuration = total
	stats.LatencyMS = float64(total) / 1e6

	evalSpan := total
	if !firstTokenAt.IsZero() && firstTokenAt.After(reqAt) && doneAt.After(firstTokenAt) {
		stats.LoadDuration = firstTokenAt.Sub(reqAt)
		evalSpan = doneAt.Sub(firstTokenAt)
		stats.EvalDuration = evalSpan
	}
	if stats.EvalCount > 0 && evalSpan > 0 {
		stats.PerSec = float64(stats.EvalCount) / evalSpan.Seconds()
	}
}

// inferenceID returns a stable id for one captured exchange, unique BOTH within
// a run (via seq) AND across runs (via runID). Cross-run uniqueness is required:
// the durable SQLite store persists ids and App.InferenceDetail looks records up
// by id, so a bare per-run counter (seq resetting to 0 each launch) made a new
// run's "inf-1" collide with a previous run's persisted record and the detail
// panel showed a ghost from an earlier session.
func inferenceID(runID string, seq int) string {
	return "inf-" + runID + "-" + strconv.Itoa(seq)
}

// newRunID returns a process-unique nonce used to namespace inference ids per
// run. It uses crypto/rand so two launches (even within the same nanosecond)
// never share a nonce; on the vanishingly rare rand failure it falls back to the
// wall-clock nanos, which is still run-unique for a single-user local tool.
func newRunID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}

// canonicalTuple returns a direction-independent key for a TCP connection so
// that the request (client→server) and response (server→client) segments —
// which the OS reports with swapped src/dst — map to the same connection. The
// two endpoints are ordered deterministically by (IP, port).
func canonicalTuple(t reassembly.FourTuple) reassembly.FourTuple {
	src := t
	swapped := reassembly.FourTuple{SrcIP: t.DstIP, DstIP: t.SrcIP, SrcPort: t.DstPort, DstPort: t.SrcPort}
	if endpointLess(swapped.SrcIP, swapped.SrcPort, src.SrcIP, src.SrcPort) {
		return swapped
	}
	return src
}

// endpointLess orders two (IP, port) endpoints deterministically.
func endpointLess(ipA string, portA uint16, ipB string, portB uint16) bool {
	if ipA != ipB {
		return ipA < ipB
	}
	return portA < portB
}

// buildCaptureInput derives the per-category signals for the projector from
// the live source status and the most recent inference value.
func buildCaptureInput(status capture.SourceStatus, current inference.Inference) dashboard.CaptureInput {
	if !status.Active {
		return dashboard.CaptureInput{
			SourceActive:   false,
			UnelevatedNote: status.Reason,
		}
	}

	hasTokens := current.Tokens != nil && current.Status == inference.PhaseCompleted
	return dashboard.CaptureInput{
		SourceActive:    true,
		HasLatency:      hasTokens && current.Tokens.LatencyMS > 0,
		HasTokenCounts:  hasTokens && (current.Tokens.EvalCount > 0 || current.Tokens.PromptEvalCount > 0),
		HasPayload:      current.PromptSize > 0,
		HasStatus:       true, // HTTP status code always captured when source is active
		HasStreamChunks: current.Streaming,
	}
}
