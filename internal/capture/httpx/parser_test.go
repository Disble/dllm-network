// Package httpx implements a pure, incremental HTTP/1.1 parser that operates
// on reassembled byte streams produced by internal/capture/reassembly. It
// handles Content-Length bodies, Transfer-Encoding: chunked bodies (including
// chunk-size lines that arrive split across multiple Feed calls), and streamed
// NDJSON bodies used by /api/generate and /api/chat.
//
// The package has ZERO dependency on any capture driver, OS API, or the
// internal/capture (source) package — it is fully unit-testable without
// elevation using golden byte fixtures in testdata/.
package httpx

import (
	"encoding/json"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- helpers ----------------------------------------------------------------

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	return data
}

// buildChunkedResponse builds a minimal HTTP/1.1 chunked response whose body
// is a single chunk containing body. Used in table-driven in-progress tests
// where we want to control whether the terminal chunk is present.
func buildChunkedResponse(body string, includeTerminal bool) []byte {
	s := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: application/x-ndjson\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n" +
		fmt.Sprintf("%x\r\n", len(body)) +
		body + "\r\n"
	if includeTerminal {
		s += "0\r\n\r\n"
	}
	return []byte(s)
}

// buildSSEResponse builds a minimal chunked text/event-stream response whose
// body carries the given SSE `data:` events in a single chunk.
func buildSSEResponse(body string) []byte {
	s := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/event-stream\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n" +
		fmt.Sprintf("%x\r\n", len(body)) +
		body + "\r\n" +
		"0\r\n\r\n"
	return []byte(s)
}

// TestParser_SSEStripsDataPrefix verifies that for a text/event-stream response
// the parser strips the `data: ` SSE prefix so downstream sees clean inner
// payloads (the OpenAI JSON and the [DONE] sentinel), and skips blank lines.
func TestParser_SSEStripsDataPrefix(t *testing.T) {
	t.Parallel()

	body := "data: {\"object\":\"chat.completion.chunk\",\"choices\":[]}\n\n" +
		"data: [DONE]\n\n"
	msgs := NewParser().Feed(buildSSEResponse(body))

	var bodies []string
	for _, m := range msgs {
		if m.Kind == KindResponse {
			bodies = append(bodies, string(m.Body))
		}
	}
	if len(bodies) != 2 {
		t.Fatalf("expected 2 SSE payloads, got %d: %v", len(bodies), bodies)
	}
	if bodies[0] != `{"object":"chat.completion.chunk","choices":[]}` {
		t.Errorf("first payload not de-prefixed: %q", bodies[0])
	}
	if bodies[1] != "[DONE]" {
		t.Errorf("second payload should be the [DONE] sentinel, got %q", bodies[1])
	}
}

// ---- 2.1 RED: Content-Length body -------------------------------------------

func TestParser_ContentLengthBody(t *testing.T) {
	t.Parallel()

	fixture := readFixture(t, "response_content_length.bin")
	p := NewParser()
	msgs := p.Feed(fixture)

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	msg := msgs[0]
	if msg.Kind != KindResponse {
		t.Fatalf("expected KindResponse, got %v", msg.Kind)
	}
	if msg.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", msg.StatusCode)
	}
	wantBody := `{"model":"llama3","done":true}`
	if string(msg.Body) != wantBody {
		t.Fatalf("body mismatch\n got: %q\nwant: %q", msg.Body, wantBody)
	}
}

// ---- 2.3 RED: Chunked multi-chunk decode ------------------------------------

func TestParser_ChunkedResponseDecodesFullBody(t *testing.T) {
	t.Parallel()

	fixture := readFixture(t, "chunked_response_multi_chunk.bin")
	p := NewParser()
	msgs := p.Feed(fixture)

	// Two NDJSON lines from two chunks → 2 messages.
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (one per NDJSON line), got %d", len(msgs))
	}
	if string(msgs[0].Body) != `{"response":"Hello"}` {
		t.Fatalf("msg[0] body mismatch: %q", msgs[0].Body)
	}
	if string(msgs[1].Body) != `{"response":" world"}` {
		t.Fatalf("msg[1] body mismatch: %q", msgs[1].Body)
	}
}

// ---- 2.5 RED: Partial chunk-size line across two feeds ----------------------

func TestParser_PartialChunkArrivesIncrementally(t *testing.T) {
	t.Parallel()

	part1 := readFixture(t, "chunked_partial_split_chunksize_line.part1.bin")
	part2 := readFixture(t, "chunked_partial_split_chunksize_line.part2.bin")

	p := NewParser()
	msgs1 := p.Feed(part1)
	msgs2 := p.Feed(part2)

	all := append(msgs1, msgs2...)

	if len(all) != 1 {
		t.Fatalf("expected 1 message across two feeds, got %d", len(all))
	}
	wantBody := `{"response":"Hi!"}`
	if string(all[0].Body) != wantBody {
		t.Fatalf("body mismatch after partial feed\n got: %q\nwant: %q", all[0].Body, wantBody)
	}
}

// ---- 2.7 RED: Multi-line NDJSON streamed response ---------------------------

func TestParser_MultiLineStreamedGenerateResponse(t *testing.T) {
	t.Parallel()

	fixture := readFixture(t, "generate_ndjson_5_lines.bin")
	p := NewParser()
	msgs := p.Feed(fixture)

	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages (one per NDJSON line), got %d", len(msgs))
	}

	// First 4 must not be terminal.
	for i, m := range msgs[:4] {
		if m.Done {
			t.Errorf("msgs[%d] must not be flagged Done", i)
		}
	}

	// Last must be terminal.
	last := msgs[4]
	if !last.Done {
		t.Fatal("last message must be flagged Done (done:true in NDJSON)")
	}

	// Verify JSON is valid and model field present.
	var obj map[string]interface{}
	if err := json.Unmarshal(last.Body, &obj); err != nil {
		t.Fatalf("last message body not valid JSON: %v", err)
	}
	if model, ok := obj["model"].(string); !ok || model != "llama3" {
		t.Fatalf("expected model=llama3, got %v", obj["model"])
	}
}

// ---- 2.9 RED: Malformed line isolation --------------------------------------

func TestParser_MalformedLineIsolatedNotFatal(t *testing.T) {
	t.Parallel()

	fixture := readFixture(t, "ndjson_one_malformed_line.bin")
	p := NewParser()
	msgs := p.Feed(fixture)

	// 3 lines: valid, malformed, valid.
	// Malformed line reported with ParseErr; parser continues for line 3.
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages (including malformed), got %d", len(msgs))
	}

	if msgs[0].ParseErr != nil {
		t.Fatalf("msgs[0] should be valid, got ParseErr: %v", msgs[0].ParseErr)
	}
	if msgs[1].ParseErr == nil {
		t.Fatal("msgs[1] should have ParseErr set for malformed JSON")
	}
	if msgs[2].ParseErr != nil {
		t.Fatalf("msgs[2] should be valid after malformed line, got ParseErr: %v", msgs[2].ParseErr)
	}
	if !msgs[2].Done {
		t.Fatal("msgs[2] should be terminal (done:true)")
	}
}

// ---- in-progress streaming: no terminal line yet ----------------------------

// TestParser_InProgressStreamingNoTerminalLine verifies that a partially fed
// NDJSON stream (no done:true yet) emits intermediate messages with Done=false
// and does not block or fabricate a terminal message.
func TestParser_InProgressStreamingNoTerminalLine(t *testing.T) {
	t.Parallel()

	lines := []string{
		`{"model":"llama3","response":"Once","done":false}`,
		`{"model":"llama3","response":" upon","done":false}`,
		`{"model":"llama3","response":" a","done":false}`,
		`{"model":"llama3","response":" time","done":false}`,
	}
	var body string
	for _, l := range lines {
		body += l + "\n"
	}
	// Build chunked response without the terminal chunk.
	raw := buildChunkedResponse(body, false)

	p := NewParser()
	msgs := p.Feed(raw)

	if len(msgs) != 4 {
		t.Fatalf("expected 4 in-progress messages, got %d", len(msgs))
	}
	for i, m := range msgs {
		if m.Done {
			t.Errorf("msgs[%d] must not be Done in in-progress stream", i)
		}
		if m.ParseErr != nil {
			t.Errorf("msgs[%d] unexpected ParseErr: %v", i, m.ParseErr)
		}
	}
}

// ---- purity gate: no driver/OS-bound imports --------------------------------

// TestParser_RunsWithoutElevation asserts this package has zero dependency on
// WinDivert, syscall, or any capture-driver API. It inspects non-test imports
// via go/build so it fails loudly on any platform without needing admin rights.
func TestParser_RunsWithoutElevation(t *testing.T) {
	t.Parallel()

	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("import package: %v", err)
	}

	forbidden := []string{"syscall", "dllm-network/internal/capture"}
	for _, imp := range pkg.Imports {
		for _, bad := range forbidden {
			if imp == bad || strings.HasPrefix(imp, bad+"/") {
				t.Fatalf("httpx package must not import %q (driver/OS-bound), got import %q", bad, imp)
			}
		}
	}
}
