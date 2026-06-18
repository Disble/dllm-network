package httpx

import "testing"

// A parsed request must expose every header in wire order with original casing.
func TestParser_ExposesRequestHeadersInOrder(t *testing.T) {
	p := NewParser()
	body := `{"model":"gemma4:12b","prompt":"hi"}`
	raw := "POST /api/generate HTTP/1.1\r\n" +
		"Host: 127.0.0.1:11434\r\n" +
		"Content-Type: application/json\r\n" +
		"User-Agent: Go-http-client/1.1\r\n" +
		"Content-Length: " + itoaLocal(len(body)) + "\r\n\r\n" + body
	msgs := p.Feed([]byte(raw))

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	got := msgs[0].Headers
	want := []Header{
		{Name: "Host", Value: "127.0.0.1:11434"},
		{Name: "Content-Type", Value: "application/json"},
		{Name: "User-Agent", Value: "Go-http-client/1.1"},
		{Name: "Content-Length", Value: itoaLocal(len(body))},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d headers, got %d: %+v", len(want), len(got), got)
	}
	for i, h := range want {
		if got[i] != h {
			t.Errorf("header[%d]: got %+v, want %+v", i, got[i], h)
		}
	}
}

// A chunked NDJSON response must carry its parsed headers on every emitted line.
func TestParser_ExposesResponseHeadersOnNDJSONLines(t *testing.T) {
	p := NewParser()
	raw := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: application/x-ndjson\r\n" +
		"Transfer-Encoding: chunked\r\n\r\n"
	// One chunk carrying a complete NDJSON line.
	line := `{"model":"gemma4:12b","response":"hi","done":false}`
	chunk := itoaHexLocal(len(line)) + "\r\n" + line + "\r\n"
	msgs := p.Feed([]byte(raw + chunk))

	if len(msgs) != 1 {
		t.Fatalf("expected 1 NDJSON message, got %d", len(msgs))
	}
	got := msgs[0].Headers
	if len(got) != 2 {
		t.Fatalf("expected 2 response headers, got %d: %+v", len(got), got)
	}
	if got[0].Name != "Content-Type" || got[0].Value != "application/x-ndjson" {
		t.Errorf("header[0]: got %+v, want Content-Type: application/x-ndjson", got[0])
	}
	if got[1].Name != "Transfer-Encoding" {
		t.Errorf("header[1]: got %+v, want Transfer-Encoding", got[1])
	}
}

// itoaHexLocal renders n as a lowercase hex string (chunk-size lines are hex).
func itoaHexLocal(n int) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789abcdef"
	var b []byte
	for n > 0 {
		b = append([]byte{digits[n%16]}, b...)
		n /= 16
	}
	return string(b)
}
