package reassembly

import "time"

// connKey uniquely identifies one direction of one connection.
type connKey struct {
	tuple FourTuple
	dir   Direction
}

// connState tracks per-connection/direction reassembly progress.
type connState struct {
	// nextSeq is the next expected stream byte offset.
	nextSeq uint32
	// pending holds segments received ahead of nextSeq, keyed by SeqNo,
	// until the gap before them is filled.
	pending map[uint32][]byte
	// lastSeen is the timestamp of the most recently pushed segment for
	// this connection/direction, used for idle eviction.
	lastSeen time.Time
}

// defaultIdleTimeout is used by New() when no explicit idle timeout is
// configured.
const defaultIdleTimeout = 30 * time.Second

// Reassembler reorders and joins captured TCP segments into contiguous
// per-connection, per-direction byte streams. Connection state is created
// lazily on first segment and evicted once idle past idleTimeout (covering
// connections whose FIN was never observed/captured).
type Reassembler struct {
	conns       map[connKey]*connState
	idleTimeout time.Duration
}

// New creates an empty Reassembler using the default idle timeout.
func New() *Reassembler {
	return NewWithIdleTimeout(defaultIdleTimeout)
}

// NewWithIdleTimeout creates an empty Reassembler with an explicit idle
// timeout, used by EvictIdle to determine which connections to drop.
func NewWithIdleTimeout(idleTimeout time.Duration) *Reassembler {
	return &Reassembler{
		conns:       make(map[connKey]*connState),
		idleTimeout: idleTimeout,
	}
}

// Push feeds a single captured segment into the reassembler and returns the
// newly contiguous bytes (a delta) made available by this segment for that
// connection/direction. Callers concatenate returned deltas, in the order
// Push was called, to reconstruct the full ordered stream — unless
// out-of-order delivery occurred, in which case deltas should be ordered by
// Stream.SeqNo before concatenation.
//
// Segments arriving out of order (SeqNo ahead of the next expected byte)
// are buffered in-memory until the preceding gap is filled, then flushed as
// a single contiguous delta. Connection state is created lazily here on
// first segment for a given 4-tuple/direction.
func (r *Reassembler) Push(seg Segment) []Stream {
	key := connKey{tuple: seg.Tuple, dir: seg.Dir}

	state, ok := r.conns[key]
	if !ok {
		state = newConnState()
		r.conns[key] = state
	}
	state.lastSeen = seg.At

	switch {
	case seg.SeqNo == state.nextSeq:
		// In-order: emit immediately, then drain any pending segments
		// that are now contiguous.
		return acceptContiguous(seg.Tuple, seg.Dir, state, seg.SeqNo, seg.Payload)
	case seg.SeqNo > state.nextSeq:
		// Out-of-order/ahead: buffer until the gap is filled.
		state.pending[seg.SeqNo] = append([]byte(nil), seg.Payload...)
		return nil
	default:
		// Already-seen byte range (duplicate/overlap) — nothing new to
		// emit.
		return nil
	}
}

// EvictIdle removes connection state for any connection/direction whose
// lastSeen timestamp is older than the reassembler's idle timeout relative
// to now. This bounds memory growth for connections whose FIN/close was
// never observed by the capture source (e.g. dropped packets, ungraceful
// client exit). After eviction, a subsequent Push for the same 4-tuple
// starts a fresh connection state — sequence numbers restart at the new
// segment's first byte, exactly like a brand new connection.
func (r *Reassembler) EvictIdle(now time.Time) {
	for key, state := range r.conns {
		if now.Sub(state.lastSeen) >= r.idleTimeout {
			delete(r.conns, key)
		}
	}
}

// newConnState creates a fresh connection state, always expecting byte
// offset 0 next — the first segment observed for a connection is not
// necessarily the first byte of the stream (out-of-order capture).
func newConnState() *connState {
	return &connState{
		nextSeq: 0,
		pending: make(map[uint32][]byte),
	}
}

// acceptContiguous emits payload as a delta starting at seqNo, advances
// nextSeq, and drains any buffered pending segments that become contiguous
// as a result.
func acceptContiguous(tuple FourTuple, dir Direction, state *connState, seqNo uint32, payload []byte) []Stream {
	delta := make([]byte, len(payload))
	copy(delta, payload)

	streams := []Stream{{Tuple: tuple, Dir: dir, Payload: delta, SeqNo: seqNo}}
	state.nextSeq = seqNo + uint32(len(payload))

	for {
		next, ok := state.pending[state.nextSeq]
		if !ok {
			break
		}
		delete(state.pending, state.nextSeq)

		flushed := make([]byte, len(next))
		copy(flushed, next)

		streams = append(streams, Stream{Tuple: tuple, Dir: dir, Payload: flushed, SeqNo: state.nextSeq})
		state.nextSeq += uint32(len(next))
	}

	return streams
}
