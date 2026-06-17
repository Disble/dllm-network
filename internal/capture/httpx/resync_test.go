package httpx

import "testing"

// When capture starts on a pre-existing keep-alive connection, the first bytes
// the parser sees are the TAIL of an earlier in-flight message (garbage from a
// message-boundary point of view). The parser MUST discard that garbage and
// resync to the next valid HTTP start-line, otherwise every subsequent message
// on that connection is mis-parsed and no exchange is ever formed.

func TestParser_ResyncsResponseAfterMidStreamGarbage(t *testing.T) {
	p := NewParser()

	// Leading garbage = tail of a previous chunked response, then a clean
	// response for the next request on the same keep-alive connection.
	garbage := "ample\"}\n\r\n0\r\n\r\n"
	clean := "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}"

	msgs := p.Feed([]byte(garbage + clean))

	var got *Message
	for i := range msgs {
		if msgs[i].Kind == KindResponse {
			got = &msgs[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("expected a resynced KindResponse, got %d msgs: %+v", len(msgs), msgs)
	}
	if got.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", got.StatusCode)
	}
}

func TestParser_ResyncsRequestAfterMidStreamGarbage(t *testing.T) {
	p := NewParser()

	garbage := "some leftover body bytes with no boundary\r\n"
	body := `{"model":"gemma4:12b"}`
	clean := "POST /api/chat HTTP/1.1\r\nContent-Length: " + itoaLocal(len(body)) + "\r\n\r\n" + body

	msgs := p.Feed([]byte(garbage + clean))

	var got *Message
	for i := range msgs {
		if msgs[i].Kind == KindRequest {
			got = &msgs[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("expected a resynced KindRequest, got %d msgs: %+v", len(msgs), msgs)
	}
	if got.Method != "POST" || got.Path != "/api/chat" {
		t.Fatalf("expected POST /api/chat, got %s %s", got.Method, got.Path)
	}
}

// Pure garbage with no start-line must not crash, emit, or grow the buffer
// without bound.
func TestParser_GarbageWithoutStartLineIsDropped(t *testing.T) {
	p := NewParser()
	msgs := p.Feed([]byte("just some random bytes\nwith newlines\nbut no http here\n"))
	if len(msgs) != 0 {
		t.Fatalf("expected no messages from garbage, got %+v", msgs)
	}
}

// A clean request split exactly at the start (no garbage) must still parse —
// resync must not discard a valid leading start-line.
func TestParser_DoesNotDiscardValidLeadingRequest(t *testing.T) {
	p := NewParser()
	msgs := p.Feed([]byte("GET /api/tags HTTP/1.1\r\nHost: x\r\n\r\n"))
	if len(msgs) != 1 || msgs[0].Method != "GET" {
		t.Fatalf("resync discarded a valid leading request: %+v", msgs)
	}
}
