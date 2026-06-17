package app

import (
	"context"
	"errors"

	"ollama-telemetry/internal/capture"
	"ollama-telemetry/internal/capture/httpx"
	"ollama-telemetry/internal/capture/reassembly"
	"ollama-telemetry/internal/dashboard"
	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

// isTerminalRecvErr returns true for errors that should cause the recv-loop to
// exit cleanly: context cancellation, deadline exceeded, or the fake source's
// exhausted sentinel.
func isTerminalRecvErr(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, capture.ErrFakeExhausted)
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

	// cancelCapture stops the capture goroutine's context.
	cancelCapture context.CancelFunc
	done          chan struct{}
}

// newInferencePipeline creates a pipeline wired to the given source, store,
// and publisher. It does NOT start the goroutine — call run().
func newInferencePipeline(
	src capture.CaptureSource,
	recent *store.Recent,
	publisher snapshotProjector,
) *inferencePipeline {
	return &inferencePipeline{
		src:       src,
		recent:    recent,
		publisher: publisher,
		extractor: inference.NewExtractor(),
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

	// Per-connection state: one Reassembler and two Parsers (request ToServer,
	// response FromServer) per 4-tuple.
	type connParsers struct {
		req  *httpx.Parser // ToServer
		resp *httpx.Parser // FromServer
	}
	reassembler := reassembly.New()
	parsers := make(map[reassembly.FourTuple]*connParsers)

	// requestBuf holds the most recently parsed request Message per tuple so
	// we can pair it with the terminal response.
	requestBuf := make(map[reassembly.FourTuple]httpx.Message)

	// current is the running inference state emitted with every snapshot.
	var current inference.Inference

	status := p.src.Status()
	capLog("recvLoop start: active=%v elevated=%v reason=%q", status.Active, status.Elevated, status.Reason)
	segCount := 0

	for {
		seg, err := p.src.Recv(ctx)
		if err != nil {
			// Context cancelled (Quit), deadline exceeded, or fake source
			// exhausted — exit cleanly.
			if isTerminalRecvErr(err) {
				capLog("recvLoop exit: %v (total segments=%d)", err, segCount)
				return
			}
			// Any other driver-level error: non-fatal — keep running.
			capLog("recv non-fatal error: %v", err)
			continue
		}
		segCount++
		if len(seg.Payload) > 0 {
			capLog("seg #%d dir=%d len=%d src=%s:%d dst=%s:%d seq=%d", segCount, seg.Dir, len(seg.Payload), seg.Tuple.SrcIP, seg.Tuple.SrcPort, seg.Tuple.DstIP, seg.Tuple.DstPort, seg.SeqNo)
		}

		// Refresh status after each recv (source may have transitioned).
		status = p.src.Status()

		// Feed segment into reassembler. capture.Segment has identical fields to
		// reassembly.Segment (FourTuple and Direction are type-aliases from the
		// reassembly package), so the conversion is field-by-field.
		streams := reassembler.Push(reassembly.Segment{
			Tuple:   seg.Tuple,
			Dir:     seg.Dir,
			Payload: seg.Payload,
			SeqNo:   seg.SeqNo,
			At:      seg.At,
		})

		for _, stream := range streams {
			// Key per logical connection, not per packet direction: the OS
			// reports the request (client→server) and response (server→client)
			// with SWAPPED 4-tuples, so we canonicalise to pair them.
			key := canonicalTuple(stream.Tuple)
			cp, ok := parsers[key]
			if !ok {
				cp = &connParsers{req: httpx.NewParser(), resp: httpx.NewParser()}
				parsers[key] = cp
			}

			var msgs []httpx.Message
			switch stream.Dir {
			case capture.DirToServer:
				msgs = cp.req.Feed(stream.Payload)
				for _, m := range msgs {
					capLog("  req msg: kind=%d method=%s path=%s", m.Kind, m.Method, m.Path)
					if m.Kind == httpx.KindRequest {
						requestBuf[key] = m
					}
				}
			case capture.DirFromServer:
				msgs = cp.resp.Feed(stream.Payload)
				for _, m := range msgs {
					capLog("  resp msg: kind=%d status=%d done=%v bodyLen=%d", m.Kind, m.StatusCode, m.Done, len(m.Body))
					if m.Kind != httpx.KindResponse {
						continue
					}
					req, hasReq := requestBuf[key]
					if !hasReq {
						capLog("    no buffered request for key %s:%d/%s:%d", key.SrcIP, key.SrcPort, key.DstIP, key.DstPort)
						continue
					}
					inf, ok := p.extractor.FromExchange(req, m)
					capLog("    FromExchange ok=%v status=%d model=%s hasTokens=%v", ok, inf.Status, inf.Model, inf.Tokens != nil)
					if !ok {
						continue
					}
					// Ignore non-inference exchanges (e.g. /api/tags, /api/ps,
					// /api/version polls — including this app's own orchestrator
					// polling). They must not overwrite the displayed inference.
					if inf.Status == inference.PhaseMetadataOnly {
						continue
					}
					current = inf
					if inf.Status == inference.PhaseCompleted {
						p.recent.RecordInferenceCompletion(inf)
					}
				}
			}
		}

		// Build CaptureInput from live source status.
		captureInput := buildCaptureInput(status, current)

		inferenceState := dashboard.InferenceState{
			Current: current,
			Recent:  p.recent.InferenceEvents(),
		}

		_, _ = p.publisher.Publish(ctx, dashboard.ProjectionInput{
			Capture:   captureInput,
			Inference: inferenceState,
		})
	}
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
