package capture_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ollama-telemetry/internal/capture"
)

// TestFakeSource_ReplaysGoldenSegments asserts that the fake CaptureSource
// replays a configured slice of Segments in order, returning them one by one
// via Recv, and then signals EOF (an error) once all segments are exhausted.
//
// The test reuses the WU1 golden byte fixtures to create realistic segment
// payloads — the same bytes that the reassembler tests validate downstream.
func TestFakeSource_ReplaysGoldenSegments(t *testing.T) {
	t.Parallel()

	// Load WU1 golden fixture bytes so our fake segments match real traffic.
	seg1Bytes := readCaptureFixture(t, "generate_3_segments.seg1.bin")
	seg2Bytes := readCaptureFixture(t, "generate_3_segments.seg2.bin")
	seg3Bytes := readCaptureFixture(t, "generate_3_segments.seg3.bin")

	tuple := capture.FourTuple{
		SrcIP:   "127.0.0.1",
		DstIP:   "127.0.0.1",
		SrcPort: 51000,
		DstPort: 11434,
	}
	base := time.Date(2026, time.June, 16, 12, 0, 0, 0, time.UTC)

	segments := []capture.Segment{
		{Tuple: tuple, Dir: capture.DirToServer, Payload: seg1Bytes, SeqNo: 0, At: base},
		{Tuple: tuple, Dir: capture.DirToServer, Payload: seg2Bytes, SeqNo: uint32(len(seg1Bytes)), At: base.Add(time.Millisecond)},
		{Tuple: tuple, Dir: capture.DirToServer, Payload: seg3Bytes, SeqNo: uint32(len(seg1Bytes) + len(seg2Bytes)), At: base.Add(2 * time.Millisecond)},
	}

	src := capture.NewFakeSource(segments)

	if err := src.Open(); err != nil {
		t.Fatalf("Open: unexpected error: %v", err)
	}

	status := src.Status()
	if !status.Active {
		t.Errorf("Status.Active: want true after Open, got false (reason: %q)", status.Reason)
	}

	ctx := context.Background()

	// Recv each segment in order; verify payload matches.
	for i, want := range segments {
		got, err := src.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv segment %d: unexpected error: %v", i, err)
		}
		if got.SeqNo != want.SeqNo {
			t.Errorf("segment %d SeqNo: want %d, got %d", i, want.SeqNo, got.SeqNo)
		}
		if got.Dir != want.Dir {
			t.Errorf("segment %d Dir: want %v, got %v", i, want.Dir, got.Dir)
		}
		if !bytes.Equal(got.Payload, want.Payload) {
			t.Errorf("segment %d Payload mismatch (got %d bytes, want %d bytes)", i, len(got.Payload), len(want.Payload))
		}
	}

	// After all segments are exhausted, Recv must return an error (EOF-like)
	// so callers know the replay is complete.
	_, err := src.Recv(ctx)
	if err == nil {
		t.Error("Recv after exhaustion: expected error (fake source exhausted), got nil")
	}

	if err := src.Close(); err != nil {
		t.Errorf("Close: unexpected error: %v", err)
	}
}

// readCaptureFixture loads a byte fixture from the WU1 reassembly testdata
// directory, relative to the repo root, so the fake source tests can reuse
// the same golden bytes without duplicating them.
func readCaptureFixture(t *testing.T, name string) []byte {
	t.Helper()
	// Fixtures live in internal/capture/reassembly/testdata/ — traverse up
	// from internal/capture/ where this test file lives.
	path := filepath.Join("..", "capture", "reassembly", "testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read capture fixture %q: %v", path, err)
	}
	return data
}
