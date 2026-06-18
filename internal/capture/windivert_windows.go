//go:build windows

package capture

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	windivertLayerNetwork = 0
	windivertFlagSniff    = 0x0001 // WINDIVERT_FLAG_SNIFF — do not drop/modify
	windivertFlagRecvOnly = 0x0004 // WINDIVERT_FLAG_RECV_ONLY (0x0008 is SEND_ONLY)
	windivertFlags        = windivertFlagSniff | windivertFlagRecvOnly // 0x0005

	windivertInvalidHandle = ^uintptr(0)
	errnoAccessDenied      = 5 // ERROR_ACCESS_DENIED — not elevated

	windivertFilter = "tcp.DstPort == 11434 or tcp.SrcPort == 11434"

	packetBufSize  = 65535
	addrBufSize    = 128 // WINDIVERT_ADDRESS struct (padded generously)
	ollamaPort     = uint16(11434)
	ipProtoTCP     = 6
)

// windivertSource is the real CaptureSource backed by WinDivert kernel driver.
// It uses pure Go syscall (syscall.NewLazyDLL) — no cgo, no gcc required.
//
// Additive design decision (WU5): NewWinDivertSource is the only file in the
// package that imports syscall. All other source implementations (noop, fake,
// stub) are driver-free. This isolates ALL OS-bound code here.
type windivertSource struct {
	mu sync.Mutex

	dll   *syscall.LazyDLL
	open  *syscall.LazyProc
	recv  *syscall.LazyProc
	close *syscall.LazyProc

	handle uintptr
	status SourceStatus

	segCh  chan Segment
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewWinDivertSource returns a CaptureSource backed by WinDivert.dll loaded
// via syscall.NewLazyDLL. The DLL must be available beside the executable
// (written there by the embed-on-first-run logic in task 5.9). If the DLL
// cannot be loaded or the process lacks administrator rights, the source
// enters graceful degradation mode: Status reports Active=false,
// Elevated=false, and a human-readable Reason — the application continues in
// poller-only mode.
//
// This function is only compiled on windows (build tag). Non-windows callers
// use windivert_stub.go which returns a noopSource.
func NewWinDivertSource() CaptureSource {
	dll := syscall.NewLazyDLL("WinDivert.dll")
	return &windivertSource{
		dll:    dll,
		open:   dll.NewProc("WinDivertOpen"),
		recv:   dll.NewProc("WinDivertRecv"),
		close:  dll.NewProc("WinDivertClose"),
		handle: windivertInvalidHandle,
		status: SourceStatus{Active: false, Elevated: false, Reason: "not opened"},
	}
}

// Open ensures WinDivert assets are written beside the exe, loads WinDivert.dll,
// calls WinDivertOpen with the Ollama filter, and starts the recv-loop goroutine.
// If the process lacks administrator rights WinDivertOpen returns
// ERROR_ACCESS_DENIED (errno 5); Open returns nil in that case and sets Status
// to {Active:false, Elevated:false, Reason:"requires administrator"} so the
// application degrades gracefully without crashing.
func (w *windivertSource) Open() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Write the embedded DLL+sys beside the exe on first run so the binary
	// is self-contained. Ignore failure (degraded status set below if DLL
	// still can't load).
	_ = EnsureWinDivertAssets()

	// Load the DLL lazily; if it is not beside the exe this fails here with
	// a clear error.
	if err := w.dll.Load(); err != nil {
		w.status = SourceStatus{
			Active:   false,
			Elevated: false,
			Reason:   fmt.Sprintf("WinDivert.dll not found: %v", err),
		}
		return nil // degraded but not fatal; caller checks Status
	}

	filter := append([]byte(windivertFilter), 0) // NUL-terminated C string
	handle, _, callErr := w.open.Call(
		uintptr(unsafe.Pointer(&filter[0])),
		windivertLayerNetwork,
		0, // priority
		uintptr(windivertFlags),
	)

	if handle == windivertInvalidHandle {
		errno, _ := callErr.(syscall.Errno)
		if uintptr(errno) == errnoAccessDenied {
			w.status = SourceStatus{
				Active:   false,
				Elevated: false,
				Reason:   "requires administrator",
			}
			return nil // graceful degradation — NOT a fatal error
		}
		w.status = SourceStatus{
			Active:   false,
			Elevated: false,
			Reason:   fmt.Sprintf("WinDivertOpen failed: %v", callErr),
		}
		return nil // still degraded-not-fatal for unknown errors too
	}

	w.handle = handle
	w.status = SourceStatus{Active: true, Elevated: true, Reason: ""}

	// Start the recv-loop goroutine that pulls packets from the driver and
	// pushes parsed Segments into segCh. The loop stops when ctx is
	// cancelled (via Close) or the handle becomes invalid.
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.segCh = make(chan Segment, 64)

	w.wg.Add(1)
	go w.recvLoop(ctx)

	return nil
}

// recvLoop runs in its own goroutine, blocking on WinDivertRecv and pushing
// parsed Segments into segCh until ctx is done or a hard driver error occurs.
func (w *windivertSource) recvLoop(ctx context.Context) {
	defer w.wg.Done()
	defer close(w.segCh)

	packet := make([]byte, packetBufSize)
	addr := make([]byte, addrBufSize)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var recvLen uint32
		r, _, _ := w.recv.Call(
			w.handle,
			uintptr(unsafe.Pointer(&packet[0])),
			uintptr(len(packet)),
			uintptr(unsafe.Pointer(&recvLen)),
			uintptr(unsafe.Pointer(&addr[0])),
		)
		if r == 0 {
			// Recv failed — check if it's because we closed the handle.
			select {
			case <-ctx.Done():
				return
			default:
				// Transient error; keep trying (driver may recover).
				continue
			}
		}

		raw := packet[:recvLen]
		seg, ok := parseIPPacket(raw, time.Now())
		if !ok {
			continue // skip malformed, ARP, non-TCP, IPv6 ext-header, etc.
		}
		if len(seg.Payload) == 0 {
			continue // skip ACK-only / empty TCP segments
		}

		select {
		case w.segCh <- seg:
		case <-ctx.Done():
			return
		}
	}
}

// Recv blocks until a Segment is available, ctx is done, or the recv-loop
// goroutine closes segCh (which happens when the source is closed).
func (w *windivertSource) Recv(ctx context.Context) (Segment, error) {
	w.mu.Lock()
	ch := w.segCh
	w.mu.Unlock()

	if ch == nil {
		// Source not yet opened or already closed — behave like noop.
		<-ctx.Done()
		return Segment{}, ctx.Err()
	}

	select {
	case <-ctx.Done():
		return Segment{}, ctx.Err()
	case seg, ok := <-ch:
		if !ok {
			// Channel closed — recv-loop exited (handle closed).
			return Segment{}, fmt.Errorf("windivert source closed")
		}
		return seg, nil
	}
}

// Close cancels the recv-loop and releases the WinDivert handle. Idempotent.
func (w *windivertSource) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}

	if w.handle != windivertInvalidHandle {
		if _, _, err := w.close.Call(w.handle); err != nil {
			return fmt.Errorf("WinDivertClose failed: %w", err)
		}
		w.handle = windivertInvalidHandle
	}

	w.wg.Wait()

	w.status = SourceStatus{Active: false, Elevated: false, Reason: "closed"}
	return nil
}

// Status returns the current operational state. Safe to call at any time.
func (w *windivertSource) Status() SourceStatus {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status
}

// --- Pure packet parsing (exported as internal helper for unit testing) -----

// parseIPv4TCPPacket parses a raw IPv4+TCP packet buffer and returns a
// Segment. It returns (Segment{}, false) for any packet that is not a valid
// IPv4/TCP frame carrying at least one byte of payload, so the caller can
// skip it without allocating.
//
// This function is purposely free of any WinDivert or syscall dependency and
// is unit-tested directly with byte fixtures in parse_test.go — no driver or
// admin rights required.
func parseIPv4TCPPacket(raw []byte, at time.Time) (Segment, bool) {
	// Minimum IPv4 header is 20 bytes.
	if len(raw) < 20 {
		return Segment{}, false
	}

	// IPv4 version/IHL byte: high nibble must be 4, low nibble = IHL in
	// 32-bit words. Validate and compute the header length.
	versionIHL := raw[0]
	if (versionIHL >> 4) != 4 {
		return Segment{}, false // not IPv4
	}
	ihl := int(versionIHL&0x0F) * 4
	if ihl < 20 || len(raw) < ihl {
		return Segment{}, false // truncated / malformed IHL
	}

	// Protocol must be TCP (6).
	if raw[9] != ipProtoTCP {
		return Segment{}, false
	}

	srcIP := net.IP(raw[12:16]).String()
	dstIP := net.IP(raw[16:20]).String()

	// TCP header starts immediately after the IPv4 header.
	return parseTCPSegment(raw[ihl:], srcIP, dstIP, at)
}

// parseIPPacket dispatches a raw IP packet to the IPv4 or IPv6 parser based on
// the version nibble. Ollama clients that connect via "localhost" frequently
// resolve to IPv6 (::1) on Windows, so capturing both is required.
func parseIPPacket(raw []byte, at time.Time) (Segment, bool) {
	if len(raw) < 1 {
		return Segment{}, false
	}
	switch raw[0] >> 4 {
	case 4:
		return parseIPv4TCPPacket(raw, at)
	case 6:
		return parseIPv6TCPPacket(raw, at)
	default:
		return Segment{}, false
	}
}

// parseIPv6TCPPacket parses a raw IPv6+TCP packet. The IPv6 header is a fixed
// 40 bytes; only plain TCP (Next Header == 6) is handled — extension headers
// are not expected on loopback Ollama traffic.
func parseIPv6TCPPacket(raw []byte, at time.Time) (Segment, bool) {
	const ipv6HeaderLen = 40
	if len(raw) < ipv6HeaderLen {
		return Segment{}, false
	}
	if (raw[0] >> 4) != 6 {
		return Segment{}, false
	}
	// Next Header (byte 6) must be TCP; extension headers are not handled.
	if raw[6] != ipProtoTCP {
		return Segment{}, false
	}

	srcIP := net.IP(raw[8:24]).String()
	dstIP := net.IP(raw[24:40]).String()

	return parseTCPSegment(raw[ipv6HeaderLen:], srcIP, dstIP, at)
}

// parseTCPSegment parses the TCP header + payload (the bytes after the IP
// header) into a Segment, shared by the IPv4 and IPv6 paths.
func parseTCPSegment(tcp []byte, srcIP, dstIP string, at time.Time) (Segment, bool) {
	if len(tcp) < 20 {
		return Segment{}, false // TCP header truncated
	}

	srcPort := binary.BigEndian.Uint16(tcp[0:2])
	dstPort := binary.BigEndian.Uint16(tcp[2:4])
	seqNo := binary.BigEndian.Uint32(tcp[4:8])

	// Data offset: high nibble of byte 12 of the TCP header, in 32-bit words.
	dataOffset := int(tcp[12]>>4) * 4
	if dataOffset < 20 || len(tcp) < dataOffset {
		return Segment{}, false // truncated TCP options
	}

	payload := tcp[dataOffset:]
	if len(payload) == 0 {
		return Segment{}, false // ACK-only; skip
	}

	// Determine direction: DstPort==11434 → client→server (ToServer),
	// SrcPort==11434 → server→client (FromServer).
	var dir Direction
	if dstPort == ollamaPort {
		dir = DirToServer
	} else if srcPort == ollamaPort {
		dir = DirFromServer
	} else {
		return Segment{}, false // not an Ollama port — filter mismatch edge case
	}

	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)

	return Segment{
		Tuple: FourTuple{
			SrcIP:   srcIP,
			DstIP:   dstIP,
			SrcPort: srcPort,
			DstPort: dstPort,
		},
		Dir:     dir,
		Payload: payloadCopy,
		SeqNo:   seqNo,
		At:      at,
	}, true
}
