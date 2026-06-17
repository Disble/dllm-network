package httpx

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// parseState tracks the parser's position within an incremental HTTP stream.
type parseState int

const (
	stateHeaders   parseState = iota // waiting for / accumulating headers
	stateBody                        // reading a Content-Length body
	stateChunkSize                   // reading a hex chunk-size line
	stateChunkData                   // reading chunk payload bytes
	stateChunkCRLF                   // consuming the CRLF after chunk payload
	stateChunkTrail                  // consuming the CRLF after the terminal "0" chunk
)

// Parser incrementally parses HTTP/1.1 request and response messages from raw
// byte feeds. Callers call Feed repeatedly as new bytes arrive; the parser
// accumulates internal state across calls and emits completed Message values
// whenever a full body (or NDJSON line) becomes available.
//
// For chunked Transfer-Encoding with NDJSON content, the parser emits one
// Message per complete NDJSON line as each chunk completes — it does NOT wait
// for the terminal "0" chunk. This matches Ollama's streaming model where each
// HTTP chunk carries one JSON object.
//
// A Parser instance is NOT safe for concurrent use from multiple goroutines.
// Create one Parser per connection direction.
type Parser struct {
	// buf accumulates raw bytes not yet consumed by the state machine.
	buf []byte

	// state is the current position in the HTTP parse lifecycle.
	state parseState

	// current holds metadata from the most recently parsed headers.
	current parsedHeaders

	// chunkRemain is the number of body bytes remaining in the current chunk
	// (used only in stateChunkData).
	chunkRemain int

	// chunkAccum accumulates bytes of the current chunk being read.
	// Flushed to output when the chunk is fully received.
	chunkAccum []byte

	// lineRemainder holds a partial NDJSON line that did not end with \n in
	// the current chunk, carried over to the next chunk.
	lineRemainder []byte
}

// parsedHeaders records the header fields we care about for a single HTTP
// message being parsed.
type parsedHeaders struct {
	kind          MessageKind
	method        string
	path          string
	statusCode    int
	contentLength int  // -1 means not set
	chunked       bool // Transfer-Encoding: chunked
	isNDJSON      bool // Content-Type: application/x-ndjson
}

// NewParser creates a ready-to-use Parser.
func NewParser() *Parser {
	return &Parser{state: stateHeaders}
}

// Feed accepts new bytes from a reassembled TCP stream and returns any
// Messages that became complete as a result of this feed. Partial messages
// are retained in internal state and completed on a subsequent Feed call.
//
// Feed never returns an error — malformed NDJSON lines are isolated (reported
// via Message.ParseErr) and parsing continues on the next line.
func (p *Parser) Feed(data []byte) []Message {
	p.buf = append(p.buf, data...)
	var out []Message

	for len(p.buf) > 0 {
		var (
			msgs     []Message
			advanced bool
		)

		switch p.state {
		case stateHeaders:
			msgs, advanced = p.consumeHeaders()
		case stateBody:
			msgs, advanced = p.consumeBody()
		case stateChunkSize:
			msgs, advanced = p.consumeChunkSize()
		case stateChunkData:
			msgs, advanced = p.consumeChunkData()
		case stateChunkCRLF:
			if len(p.buf) < 2 {
				return out
			}
			p.buf = p.buf[2:] // consume \r\n after chunk payload
			p.state = stateChunkSize
			advanced = true
		case stateChunkTrail:
			// After terminal "0\r\n" we need one more "\r\n".
			if len(p.buf) < 2 {
				return out
			}
			p.buf = p.buf[2:] // consume \r\n
			// Flush any remaining lineRemainder as a final NDJSON line.
			if len(p.lineRemainder) > 0 {
				msgs = append(msgs, p.emitNDJSONLine(p.lineRemainder))
				p.lineRemainder = nil
			}
			p.resetChunkState()
			p.state = stateHeaders
			advanced = true
		}

		out = append(out, msgs...)
		if !advanced {
			break
		}
	}

	return out
}

// consumeHeaders tries to find the end of the HTTP header section (\r\n\r\n).
// Returns any messages produced (none from headers alone) and whether progress
// was made (i.e. headers were fully consumed).
func (p *Parser) consumeHeaders() ([]Message, bool) {
	// Resync: when capture attaches to a pre-existing keep-alive connection the
	// first bytes are the tail of an earlier message. Discard anything that is
	// not the start of a valid HTTP start-line so we lock onto the next message.
	if !p.ensureStartLine() {
		return nil, false // only garbage so far — wait for more bytes
	}

	sep := []byte("\r\n\r\n")
	idx := bytes.Index(p.buf, sep)
	if idx < 0 {
		return nil, false // incomplete headers
	}

	headerBlock := p.buf[:idx]
	p.buf = p.buf[idx+4:]

	ph := parseHeaderBlock(headerBlock)
	p.current = ph

	if ph.chunked {
		p.resetChunkState()
		p.state = stateChunkSize
	} else if ph.contentLength >= 0 {
		p.state = stateBody
	} else {
		// No body framing (no Content-Length, not chunked). A request with no
		// framing has no body (e.g. GET, or a keep-alive poll) — emit it now so
		// it can be paired with its response. Without this, bodyless requests
		// are silently dropped and no exchange is ever paired. For a response
		// with no framing, body length is unknown — skip to the next message.
		p.state = stateHeaders
		if ph.kind == KindRequest {
			return []Message{{Kind: ph.kind, Method: ph.method, Path: ph.path}}, true
		}
	}

	return nil, true
}

// ensureStartLine discards leading bytes that are not the beginning of a valid
// HTTP start-line (request-line or status-line), enabling the parser to resync
// after attaching to a connection mid-stream. Returns true when the buffer
// begins at — or at a plausible partial prefix of — a start-line; false when
// only garbage is present so far and the caller should wait for more bytes.
func (p *Parser) ensureStartLine() bool {
	for {
		if len(p.buf) == 0 {
			return false
		}
		nl := bytes.IndexByte(p.buf, '\n')
		complete := nl >= 0
		line := p.buf
		if complete {
			line = p.buf[:nl]
		}
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}

		if isStartLine(line, complete) {
			return true
		}
		if !complete {
			// Garbage with no line terminator yet — drop it to bound memory.
			p.buf = p.buf[:0]
			return false
		}
		// Complete garbage line — discard it (with its \n) and re-check.
		p.buf = p.buf[nl+1:]
	}
}

// isStartLine reports whether line is, or could be a partial prefix of, a valid
// HTTP request-line ("METHOD SP target SP HTTP/x.y") or status-line ("HTTP/x.y
// CODE ..."). When complete is false the line may be only partially received.
func isStartLine(line []byte, complete bool) bool {
	// Status-line.
	if bytes.HasPrefix(line, []byte("HTTP/")) {
		return true
	}
	if !complete && isBytePrefix(line, []byte("HTTP/")) {
		return true // partial "HTTP/" prefix — wait for the rest
	}

	// Request-line.
	sp := bytes.IndexByte(line, ' ')
	if sp < 0 {
		// No space yet: only plausible as a partial uppercase method token.
		return !complete && len(line) > 0 && len(line) <= 8 && isUpperAlpha(line)
	}
	method := line[:sp]
	if len(method) == 0 || len(method) > 8 || !isUpperAlpha(method) {
		return false
	}
	if complete {
		return bytes.Contains(line, []byte(" HTTP/"))
	}
	return true // valid method + space, rest still arriving
}

func isUpperAlpha(b []byte) bool {
	for _, c := range b {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

// isBytePrefix reports whether p is a prefix of full.
func isBytePrefix(p, full []byte) bool {
	return len(p) <= len(full) && bytes.Equal(p, full[:len(p)])
}

// consumeBody reads a Content-Length body and emits a single Message.
func (p *Parser) consumeBody() ([]Message, bool) {
	cl := p.current.contentLength
	if len(p.buf) < cl {
		return nil, false // need more data
	}

	body := make([]byte, cl)
	copy(body, p.buf[:cl])
	p.buf = p.buf[cl:]

	msg := Message{
		Kind:       p.current.kind,
		Method:     p.current.method,
		Path:       p.current.path,
		StatusCode: p.current.statusCode,
		Body:       body,
	}
	p.state = stateHeaders
	return []Message{msg}, true
}

// consumeChunkSize reads the hex chunk-size line ("N\r\n"), handling the case
// where the line is split across multiple Feed calls.
func (p *Parser) consumeChunkSize() ([]Message, bool) {
	idx := bytes.Index(p.buf, []byte("\r\n"))
	if idx < 0 {
		return nil, false // chunk-size line not yet complete — wait for more data
	}

	line := strings.TrimSpace(string(p.buf[:idx]))
	// Strip any chunk extensions (";ext=val").
	if semi := strings.IndexByte(line, ';'); semi >= 0 {
		line = line[:semi]
	}

	size, err := strconv.ParseInt(line, 16, 64)
	if err != nil {
		// Malformed chunk-size — skip to next header boundary.
		p.buf = p.buf[idx+2:]
		p.state = stateHeaders
		return nil, true
	}

	p.buf = p.buf[idx+2:] // consume size line + \r\n

	if size == 0 {
		// Terminal chunk — consume trailing \r\n then emit any remainder.
		p.state = stateChunkTrail
		return nil, true
	}

	p.chunkRemain = int(size)
	p.chunkAccum = p.chunkAccum[:0]
	p.state = stateChunkData
	return nil, true
}

// consumeChunkData reads chunk payload bytes up to chunkRemain. When the
// chunk is fully received, it splits the payload into NDJSON lines and emits
// one Message per line (streaming semantics). If a line has no trailing \n,
// it is carried in lineRemainder to be prepended to the next chunk's data.
func (p *Parser) consumeChunkData() ([]Message, bool) {
	if len(p.buf) == 0 {
		return nil, false
	}

	take := p.chunkRemain
	if take > len(p.buf) {
		take = len(p.buf)
	}

	p.chunkAccum = append(p.chunkAccum, p.buf[:take]...)
	p.buf = p.buf[take:]
	p.chunkRemain -= take

	if p.chunkRemain > 0 {
		return nil, false // still need more bytes for this chunk
	}

	// Chunk fully received — emit NDJSON lines from it.
	msgs := p.emitChunkAsNDJSON(p.chunkAccum)
	p.chunkAccum = p.chunkAccum[:0]
	p.state = stateChunkCRLF
	return msgs, true
}

// emitChunkAsNDJSON splits the chunk payload by newlines and emits one Message
// per complete NDJSON line.
//
// Ollama's streaming model delivers one JSON object per HTTP chunk, optionally
// terminated with a newline. This function handles three cases:
//
//  1. Chunk ends with \n — split by lines, emit each complete line. Any
//     partial line (content after the last \n) is carried in lineRemainder.
//
//  2. Chunk has no \n at all — the entire chunk (prepended with any existing
//     lineRemainder) is treated as one complete JSON object and emitted
//     immediately. This is the common Ollama case where each chunk IS one
//     NDJSON message.
//
//  3. Chunk has internal \n(s) but does not end with \n — emit all
//     newline-terminated lines; carry the trailing non-terminated content in
//     lineRemainder to be joined with the next chunk.
func (p *Parser) emitChunkAsNDJSON(chunk []byte) []Message {
	// Prepend any carried-over partial line from previous chunk.
	if len(p.lineRemainder) > 0 {
		full := make([]byte, 0, len(p.lineRemainder)+len(chunk))
		full = append(full, p.lineRemainder...)
		full = append(full, chunk...)
		chunk = full
		p.lineRemainder = nil
	}

	// Case 2: no newline in chunk at all — emit the whole chunk as one message.
	if !bytes.ContainsRune(chunk, '\n') {
		trimmed := bytes.TrimSpace(chunk)
		if len(trimmed) == 0 {
			return nil
		}
		return []Message{p.emitNDJSONLine(trimmed)}
	}

	// Cases 1 & 3: scan newline-delimited lines.
	var out []Message
	sc := bufio.NewScanner(bytes.NewReader(chunk))
	for sc.Scan() {
		line := sc.Bytes()
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		out = append(out, p.emitNDJSONLine(trimmed))
	}

	// If chunk does NOT end with \n, the last scanned line is actually a
	// partial line that should wait for more data. Remove the last emitted
	// message and carry its raw bytes into lineRemainder.
	if chunk[len(chunk)-1] != '\n' {
		lastNL := bytes.LastIndexByte(chunk, '\n')
		partial := chunk[lastNL+1:]
		p.lineRemainder = append([]byte(nil), bytes.TrimSpace(partial)...)
		if len(out) > 0 {
			out = out[:len(out)-1]
		}
	}

	return out
}

// emitNDJSONLine parses a single NDJSON line and returns a Message.
// Malformed JSON is reported via Message.ParseErr (isolation semantics).
func (p *Parser) emitNDJSONLine(line []byte) Message {
	msg := Message{
		Kind:       p.current.kind,
		Method:     p.current.method,
		Path:       p.current.path,
		StatusCode: p.current.statusCode,
		Body:       append([]byte(nil), line...),
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(line, &obj); err != nil {
		msg.ParseErr = fmt.Errorf("ndjson parse: %w", err)
	} else {
		if raw, ok := obj["done"]; ok {
			var done bool
			if jsonErr := json.Unmarshal(raw, &done); jsonErr == nil {
				msg.Done = done
			}
		}
	}

	return msg
}

// resetChunkState clears accumulated chunk state for the next HTTP message.
func (p *Parser) resetChunkState() {
	p.chunkAccum = p.chunkAccum[:0]
	p.lineRemainder = nil
	p.chunkRemain = 0
}

// ---- header parsing ---------------------------------------------------------

// parseHeaderBlock parses the raw header block (everything before the blank line).
func parseHeaderBlock(block []byte) parsedHeaders {
	ph := parsedHeaders{contentLength: -1}

	lines := bytes.Split(block, []byte("\r\n"))
	if len(lines) == 0 {
		return ph
	}

	// Parse request or response line.
	firstLine := string(lines[0])
	switch {
	case strings.HasPrefix(firstLine, "HTTP/"):
		ph.kind = KindResponse
		parts := strings.SplitN(firstLine, " ", 3)
		if len(parts) >= 2 {
			code, err := strconv.Atoi(parts[1])
			if err == nil {
				ph.statusCode = code
			}
		}
	default:
		ph.kind = KindRequest
		parts := strings.SplitN(firstLine, " ", 3)
		if len(parts) >= 2 {
			ph.method = parts[0]
			ph.path = parts[1]
		}
	}

	// Parse header fields.
	for _, raw := range lines[1:] {
		line := string(raw)
		idx := strings.IndexByte(line, ':')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])

		switch key {
		case "content-length":
			n, err := strconv.Atoi(val)
			if err == nil {
				ph.contentLength = n
			}
		case "transfer-encoding":
			if strings.EqualFold(val, "chunked") {
				ph.chunked = true
			}
		case "content-type":
			lv := strings.ToLower(val)
			if strings.Contains(lv, "ndjson") || strings.Contains(lv, "x-ndjson") {
				ph.isNDJSON = true
			}
		}
	}

	return ph
}
