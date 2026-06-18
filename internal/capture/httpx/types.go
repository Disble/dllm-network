// Package httpx implements a pure, incremental HTTP/1.1 parser that operates
// on reassembled byte streams produced by internal/capture/reassembly. It
// supports Content-Length bodies, Transfer-Encoding: chunked bodies (with
// chunk-size lines that may arrive split across multiple Feed calls), and
// streamed NDJSON responses used by Ollama's /api/generate and /api/chat
// endpoints.
//
// The parser has ZERO dependency on any capture driver or OS API. It is fully
// unit-testable without elevation using golden byte fixtures.
package httpx

// Header is one HTTP header field, preserved in wire order with its original
// name casing. Duplicate names are allowed (e.g. multiple Set-Cookie) — they
// are emitted as separate entries, mirroring how Chrome DevTools renders them.
type Header struct {
	Name  string
	Value string
}

// MessageKind distinguishes whether a parsed Message originated from an HTTP
// request or an HTTP response.
type MessageKind int

const (
	// KindRequest represents a parsed HTTP request (method + path + headers + body).
	KindRequest MessageKind = iota
	// KindResponse represents a parsed HTTP response (status line + headers + body).
	KindResponse
)

// Message is one parsed HTTP entity emitted by Parser.Feed. For NDJSON
// streaming responses, each newline-delimited JSON object becomes its own
// Message rather than one large Message per HTTP response. Body contains the
// raw bytes of the message body (or a single NDJSON line for streamed
// responses). ParseErr is non-nil when the NDJSON line could not be decoded as
// JSON — the parser continues emitting subsequent lines regardless (isolation
// semantics). Done is true when the Body JSON contains "done":true (Ollama
// terminal NDJSON line).
type Message struct {
	Kind MessageKind

	// Request fields — populated when Kind == KindRequest.
	Method string
	Path   string

	// Response fields — populated when Kind == KindResponse.
	StatusCode int

	// Headers carries every parsed header field in wire order. For NDJSON
	// streaming responses, each emitted line Message carries the same response
	// headers parsed from the status-line block.
	Headers []Header

	// Body is the raw body bytes. For NDJSON streams this is a single line
	// (newline NOT included).
	Body []byte

	// Done is true when this Message represents the terminal NDJSON line
	// (i.e. the JSON object contains "done":true). Always false for
	// Content-Length or non-NDJSON chunked responses.
	Done bool

	// ParseErr is set when the NDJSON line in Body could not be parsed as
	// JSON. The Body bytes are preserved verbatim. Subsequent lines continue
	// to be emitted normally.
	ParseErr error
}
