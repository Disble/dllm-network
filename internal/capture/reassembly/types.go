// Package reassembly implements pure TCP stream reassembly for captured
// Ollama HTTP traffic. It has zero dependency on any capture driver or OS
// API — it operates only on byte fixtures and is fully unit-testable
// without elevation.
package reassembly

import "time"

// FourTuple identifies a single TCP connection by its source/destination
// IP and port pair.
type FourTuple struct {
	SrcIP   string
	DstIP   string
	SrcPort uint16
	DstPort uint16
}

// Direction identifies which side of the connection a segment travelled.
type Direction int

const (
	// DirToServer marks a segment travelling toward the Ollama server
	// (destination port 11434).
	DirToServer Direction = iota
	// DirFromServer marks a segment travelling from the Ollama server
	// back to the client (source port 11434).
	DirFromServer
)

// Segment is a single captured TCP payload, scoped to one connection and
// direction, as observed by a capture source. SeqNo is the byte offset of
// Payload within the logical stream for that connection/direction — it
// lets the reassembler detect and correct out-of-order arrival. A zero
// value is treated as "next expected byte" for sources that cannot supply
// real sequence numbers and always deliver segments in order.
type Segment struct {
	Tuple   FourTuple
	Dir     Direction
	Payload []byte
	SeqNo   uint32
	At      time.Time
}

// Stream is a contiguous, ordered byte run produced by the reassembler for
// a given connection and direction. SeqNo is the starting byte offset of
// Payload within the logical stream.
type Stream struct {
	Tuple   FourTuple
	Dir     Direction
	Payload []byte
	SeqNo   uint32
}
