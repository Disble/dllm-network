package httpx

import "testing"

// A bodyless request (GET, or any request with no Content-Length and not
// chunked) MUST still emit a KindRequest message. The original parser only
// emitted requests that carried a Content-Length body, so GET polls and any
// bodyless request were silently dropped — leaving responses unpaired and no
// inference ever extracted from real keep-alive traffic.
func TestParser_EmitsBodylessGetRequest(t *testing.T) {
	p := NewParser()
	msgs := p.Feed([]byte("GET /api/tags HTTP/1.1\r\nHost: 127.0.0.1:11434\r\nUser-Agent: Go-http-client/1.1\r\n\r\n"))

	if len(msgs) != 1 {
		t.Fatalf("expected 1 request message, got %d", len(msgs))
	}
	if msgs[0].Kind != KindRequest {
		t.Fatalf("expected KindRequest, got %d", msgs[0].Kind)
	}
	if msgs[0].Method != "GET" || msgs[0].Path != "/api/tags" {
		t.Fatalf("expected GET /api/tags, got %s %s", msgs[0].Method, msgs[0].Path)
	}
}

// Keep-alive connections send sequential requests on one stream. Each must emit
// its own request message from a single Parser instance.
func TestParser_EmitsSequentialKeepAliveRequests(t *testing.T) {
	p := NewParser()

	first := p.Feed([]byte("GET /api/version HTTP/1.1\r\nHost: x\r\n\r\n"))
	second := p.Feed([]byte("GET /api/ps HTTP/1.1\r\nHost: x\r\n\r\n"))

	if len(first) != 1 || first[0].Path != "/api/version" {
		t.Fatalf("expected first request /api/version, got %+v", first)
	}
	if len(second) != 1 || second[0].Path != "/api/ps" {
		t.Fatalf("expected second request /api/ps, got %+v", second)
	}
}

// A POST request WITH a Content-Length body must still emit (regression guard
// for the path that already worked).
func TestParser_EmitsPostRequestWithBody(t *testing.T) {
	p := NewParser()
	body := `{"model":"gemma4:12b","prompt":"hi"}`
	raw := "POST /api/generate HTTP/1.1\r\nContent-Length: " +
		itoaLocal(len(body)) + "\r\n\r\n" + body
	msgs := p.Feed([]byte(raw))

	if len(msgs) != 1 || msgs[0].Kind != KindRequest || msgs[0].Path != "/api/generate" {
		t.Fatalf("expected POST /api/generate request, got %+v", msgs)
	}
	if string(msgs[0].Body) != body {
		t.Fatalf("expected body %q, got %q", body, msgs[0].Body)
	}
}

func itoaLocal(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
