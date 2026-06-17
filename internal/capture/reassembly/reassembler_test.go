package reassembly

import (
	"bytes"
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}

	return data
}

func testTuple() FourTuple {
	return FourTuple{
		SrcIP:   "127.0.0.1",
		DstIP:   "127.0.0.1",
		SrcPort: 51000,
		DstPort: 11434,
	}
}

func TestReassembler_SingleRequestSplitAcrossPackets(t *testing.T) {
	t.Parallel()

	want := readFixture(t, "generate_full.bin")
	seg1 := readFixture(t, "generate_3_segments.seg1.bin")
	seg2 := readFixture(t, "generate_3_segments.seg2.bin")
	seg3 := readFixture(t, "generate_3_segments.seg3.bin")

	tuple := testTuple()
	base := time.Date(2026, time.June, 16, 12, 0, 0, 0, time.UTC)

	r := New()

	// Push returns the newly available contiguous bytes for this
	// connection/direction since the previous Push call (a delta), not a
	// full resend of everything seen so far. Callers concatenate deltas in
	// arrival order to reconstruct the full stream. SeqNo tracks the byte
	// offset within the logical stream so the reassembler can detect gaps
	// and out-of-order arrival.
	seq1 := uint32(0)
	seq2 := seq1 + uint32(len(seg1))
	seq3 := seq2 + uint32(len(seg2))

	var streams []Stream
	streams = append(streams, r.Push(Segment{Tuple: tuple, Dir: DirToServer, Payload: seg1, SeqNo: seq1, At: base})...)
	streams = append(streams, r.Push(Segment{Tuple: tuple, Dir: DirToServer, Payload: seg2, SeqNo: seq2, At: base.Add(time.Millisecond)})...)
	streams = append(streams, r.Push(Segment{Tuple: tuple, Dir: DirToServer, Payload: seg3, SeqNo: seq3, At: base.Add(2 * time.Millisecond)})...)

	got := concatPayloads(streams)
	if !bytes.Equal(got, want) {
		t.Fatalf("reassembled stream mismatch\n got: %q\nwant: %q", got, want)
	}
}

func concatPayloads(streams []Stream) []byte {
	var buf bytes.Buffer
	for _, s := range streams {
		buf.Write(s.Payload)
	}
	return buf.Bytes()
}

func TestReassembler_OutOfOrderSegmentArrival(t *testing.T) {
	t.Parallel()

	want := readFixture(t, "generate_full.bin")
	seg1 := readFixture(t, "out_of_order_2_segments.seg1.bin")
	seg2 := readFixture(t, "out_of_order_2_segments.seg2.bin")

	tuple := testTuple()
	base := time.Date(2026, time.June, 16, 12, 0, 0, 0, time.UTC)

	r := New()

	// Segment 2 is captured (arrives) before segment 1 — out-of-order at
	// the capture layer. SeqNo carries the original byte offset so the
	// reassembler can still produce the correct contiguous stream once
	// segment 1 fills the gap.
	var streams []Stream
	streams = append(streams, r.Push(Segment{
		Tuple: tuple, Dir: DirToServer, Payload: seg2, SeqNo: uint32(len(seg1)), At: base,
	})...)
	streams = append(streams, r.Push(Segment{
		Tuple: tuple, Dir: DirToServer, Payload: seg1, SeqNo: 0, At: base.Add(time.Millisecond),
	})...)

	got := concatOrderedPayloads(streams)
	if !bytes.Equal(got, want) {
		t.Fatalf("reassembled out-of-order stream mismatch\n got: %q\nwant: %q", got, want)
	}
}

// concatOrderedPayloads concatenates streams ordered by their starting
// sequence number, since out-of-order delivery means Push call order no
// longer matches byte order.
func concatOrderedPayloads(streams []Stream) []byte {
	sorted := make([]Stream, len(streams))
	copy(sorted, streams)

	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j-1].SeqNo > sorted[j].SeqNo; j-- {
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}

	var buf bytes.Buffer
	for _, s := range sorted {
		buf.Write(s.Payload)
	}
	return buf.Bytes()
}

// TestReassembler_RunsWithoutElevation is a meta-test asserting this
// package has zero dependency on WinDivert, syscall, or any other
// driver/OS-bound API. It inspects the package's own non-test imports via
// go/build rather than relying on runtime behavior, so it fails loudly the
// moment a driver dependency creeps in, on any platform, without needing
// administrator privileges to run.
//
// This package's tests are run as a normal (non-elevated) user in CI and
// locally: `go test ./internal/capture/reassembly/...` requires no admin
// rights, no WinDivert.dll, and no driver of any kind, because reassembly
// operates purely on in-memory byte fixtures.
func TestReassembler_EvictsIdleConnectionAndRestartsCleanly(t *testing.T) {
	t.Parallel()

	seg1 := readFixture(t, "generate_3_segments.seg1.bin")
	seg2 := readFixture(t, "generate_3_segments.seg2.bin")

	tuple := testTuple()
	base := time.Date(2026, time.June, 16, 12, 0, 0, 0, time.UTC)

	r := NewWithIdleTimeout(5 * time.Second)

	// First exchange on this 4-tuple: only the first half arrives, then
	// the connection goes idle past the timeout (e.g. client vanished,
	// FIN never observed). The reassembler must evict that connection's
	// state rather than holding it (and any pending out-of-order buffer)
	// forever.
	r.Push(Segment{Tuple: tuple, Dir: DirToServer, Payload: seg1, SeqNo: 0, At: base})

	idleCutoff := base.Add(10 * time.Second)
	r.EvictIdle(idleCutoff)

	// A new exchange reuses the same 4-tuple (e.g. a new TCP connection on
	// an ephemeral port that happened to be recycled, or — more commonly —
	// the same logical flow restarting). Because the prior state was
	// evicted, sequence numbers start fresh at 0 again and must be
	// accepted as in-order, not buffered as "out of order" against the
	// stale nextSeq from the evicted connection.
	got := r.Push(Segment{Tuple: tuple, Dir: DirToServer, Payload: seg2, SeqNo: 0, At: idleCutoff.Add(time.Millisecond)})

	if len(got) != 1 {
		t.Fatalf("expected exactly 1 stream delta after eviction+restart, got %d", len(got))
	}
	if !bytes.Equal(got[0].Payload, seg2) {
		t.Fatalf("expected fresh-connection payload after eviction\n got: %q\nwant: %q", got[0].Payload, seg2)
	}
}

func TestReassembler_RunsWithoutElevation(t *testing.T) {
	t.Parallel()

	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("import package: %v", err)
	}

	forbidden := []string{"syscall", "ollama-telemetry/internal/capture"}

	for _, imp := range pkg.Imports {
		for _, bad := range forbidden {
			if imp == bad || strings.HasPrefix(imp, bad+"/") {
				t.Fatalf("reassembly package must not import %q (driver/OS-bound dependency), got import %q", bad, imp)
			}
		}
	}
}
