//go:build windows

package capture

import (
	"encoding/binary"
	"testing"
	"time"
)

// buildIPv4TCPPacket constructs a minimal raw IPv4+TCP packet byte slice with
// the given fields. Used to build deterministic fixtures for table-driven
// parse tests — no driver, no syscall, no admin.
func buildIPv4TCPPacket(
	srcIP, dstIP [4]byte,
	srcPort, dstPort uint16,
	seqNo uint32,
	payload []byte,
) []byte {
	ihl := 20          // IPv4 header, no options
	tcpDataOffset := 5 // TCP header = 5 * 4 = 20 bytes, no options
	tcpHeaderLen := tcpDataOffset * 4
	totalLen := ihl + tcpHeaderLen + len(payload)

	pkt := make([]byte, totalLen)

	// --- IPv4 header (20 bytes) ---
	pkt[0] = 0x45                                        // version=4, IHL=5
	binary.BigEndian.PutUint16(pkt[2:4], uint16(totalLen)) // total length
	pkt[8] = 64                                          // TTL
	pkt[9] = ipProtoTCP                                  // protocol = TCP
	copy(pkt[12:16], srcIP[:])
	copy(pkt[16:20], dstIP[:])

	// --- TCP header (20 bytes, starting at ihl) ---
	tcp := pkt[ihl:]
	binary.BigEndian.PutUint16(tcp[0:2], srcPort)
	binary.BigEndian.PutUint16(tcp[2:4], dstPort)
	binary.BigEndian.PutUint32(tcp[4:8], seqNo)
	tcp[12] = byte(tcpDataOffset << 4) // data offset in high nibble

	// --- Payload ---
	copy(tcp[tcpHeaderLen:], payload)

	return pkt
}

var (
	loopback   = [4]byte{127, 0, 0, 1}
	ollamaPort16 = uint16(11434)
	clientPort = uint16(51000)
)

func TestParseIPv4TCPPacket_ToServer(t *testing.T) {
	t.Parallel()

	payload := []byte("GET /api/tags HTTP/1.1\r\nHost: 127.0.0.1:11434\r\n\r\n")
	pkt := buildIPv4TCPPacket(loopback, loopback, clientPort, ollamaPort16, 1000, payload)

	at := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	seg, ok := parseIPv4TCPPacket(pkt, at)
	if !ok {
		t.Fatal("parseIPv4TCPPacket returned ok=false for a valid ToServer packet")
	}

	if seg.Dir != DirToServer {
		t.Errorf("Dir: want DirToServer, got %v", seg.Dir)
	}
	if seg.Tuple.SrcPort != clientPort {
		t.Errorf("SrcPort: want %d, got %d", clientPort, seg.Tuple.SrcPort)
	}
	if seg.Tuple.DstPort != ollamaPort16 {
		t.Errorf("DstPort: want %d, got %d", ollamaPort16, seg.Tuple.DstPort)
	}
	if seg.SeqNo != 1000 {
		t.Errorf("SeqNo: want 1000, got %d", seg.SeqNo)
	}
	if string(seg.Payload) != string(payload) {
		t.Errorf("Payload mismatch:\n got: %q\nwant: %q", seg.Payload, payload)
	}
	if !seg.At.Equal(at) {
		t.Errorf("At: want %v, got %v", at, seg.At)
	}
}

func TestParseIPv4TCPPacket_FromServer(t *testing.T) {
	t.Parallel()

	payload := []byte("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n{}")
	pkt := buildIPv4TCPPacket(loopback, loopback, ollamaPort16, clientPort, 2000, payload)

	at := time.Now()
	seg, ok := parseIPv4TCPPacket(pkt, at)
	if !ok {
		t.Fatal("parseIPv4TCPPacket returned ok=false for a valid FromServer packet")
	}

	if seg.Dir != DirFromServer {
		t.Errorf("Dir: want DirFromServer, got %v", seg.Dir)
	}
	if seg.Tuple.SrcPort != ollamaPort16 {
		t.Errorf("SrcPort: want %d, got %d", ollamaPort16, seg.Tuple.SrcPort)
	}
	if seg.Tuple.DstPort != clientPort {
		t.Errorf("DstPort: want %d, got %d", clientPort, seg.Tuple.DstPort)
	}
	if seg.SeqNo != 2000 {
		t.Errorf("SeqNo: want 2000, got %d", seg.SeqNo)
	}
}

func TestParseIPv4TCPPacket_TableDriven(t *testing.T) {
	t.Parallel()

	type tc struct {
		name      string
		pkt       []byte
		wantOK    bool
		wantDir   Direction
		wantSeqNo uint32
	}

	payload := []byte("hello ollama")

	cases := []tc{
		{
			name:      "valid ToServer",
			pkt:       buildIPv4TCPPacket(loopback, loopback, clientPort, ollamaPort16, 0, payload),
			wantOK:    true,
			wantDir:   DirToServer,
			wantSeqNo: 0,
		},
		{
			name:      "valid FromServer",
			pkt:       buildIPv4TCPPacket(loopback, loopback, ollamaPort16, clientPort, 42, payload),
			wantOK:    true,
			wantDir:   DirFromServer,
			wantSeqNo: 42,
		},
		{
			name:   "too short (< 20 bytes)",
			pkt:    []byte{0x45, 0x00, 0x00, 0x14},
			wantOK: false,
		},
		{
			name:   "non-IPv4 version",
			pkt:    buildNonIPv4Packet(payload),
			wantOK: false,
		},
		{
			name:   "non-TCP protocol",
			pkt:    buildUDPPacket(loopback, loopback, clientPort, ollamaPort16, payload),
			wantOK: false,
		},
		{
			name:   "empty payload (ACK-only)",
			pkt:    buildIPv4TCPPacket(loopback, loopback, clientPort, ollamaPort16, 0, nil),
			wantOK: false,
		},
		{
			name:   "neither port is Ollama",
			pkt:    buildIPv4TCPPacket(loopback, loopback, 9000, 8080, 0, payload),
			wantOK: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seg, ok := parseIPv4TCPPacket(tc.pkt, time.Now())
			if ok != tc.wantOK {
				t.Fatalf("ok: want %v, got %v (seg=%+v)", tc.wantOK, ok, seg)
			}
			if !tc.wantOK {
				return
			}
			if seg.Dir != tc.wantDir {
				t.Errorf("Dir: want %v, got %v", tc.wantDir, seg.Dir)
			}
			if seg.SeqNo != tc.wantSeqNo {
				t.Errorf("SeqNo: want %d, got %d", tc.wantSeqNo, seg.SeqNo)
			}
		})
	}
}

// buildNonIPv4Packet returns a packet with IPv6 version nibble.
func buildNonIPv4Packet(payload []byte) []byte {
	pkt := buildIPv4TCPPacket(loopback, loopback, clientPort, ollamaPort16, 0, payload)
	pkt[0] = 0x65 // version = 6 (IPv6)
	return pkt
}

// buildUDPPacket returns a packet with protocol=UDP (17) instead of TCP (6).
func buildUDPPacket(srcIP, dstIP [4]byte, srcPort, dstPort uint16, payload []byte) []byte {
	pkt := buildIPv4TCPPacket(srcIP, dstIP, srcPort, dstPort, 0, payload)
	pkt[9] = 17 // UDP
	return pkt
}
