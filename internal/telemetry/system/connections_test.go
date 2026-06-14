package system

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestConnectionCollectorCollectShapesOwnedLoopbackConnections(t *testing.T) {
	t.Parallel()

	observedAt := time.Date(2026, time.June, 14, 21, 10, 0, 0, time.UTC)
	tests := []struct {
		name        string
		ownerPID    int32
		connections []ConnectionSample
		wantCount   int
		wantRemote  string
	}{
		{
			name:     "filters to owned ipv4 loopback traffic",
			ownerPID: 4242,
			connections: []ConnectionSample{
				{PID: 4242, LocalAddress: "127.0.0.1", LocalPort: 11434, RemoteAddress: "127.0.0.1", RemotePort: 55100, State: "ESTABLISHED"},
				{PID: 9999, LocalAddress: "127.0.0.1", LocalPort: 11434, RemoteAddress: "127.0.0.1", RemotePort: 55101, State: "ESTABLISHED"},
				{PID: 4242, LocalAddress: "0.0.0.0", LocalPort: 11434, RemoteAddress: "10.0.0.8", RemotePort: 55102, State: "LISTEN"},
			},
			wantCount:  1,
			wantRemote: "127.0.0.1",
		},
		{
			name:     "keeps ipv6 loopback ownership",
			ownerPID: 777,
			connections: []ConnectionSample{
				{PID: 777, LocalAddress: "::1", LocalPort: 11434, RemoteAddress: "::1", RemotePort: 55103, State: "ESTABLISHED"},
				{PID: 777, LocalAddress: "192.168.0.10", LocalPort: 11434, RemoteAddress: "192.168.0.12", RemotePort: 55104, State: "ESTABLISHED"},
			},
			wantCount:  1,
			wantRemote: "::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewConnectionCollector(fakeConnectionProvider{connections: tt.connections}, fixedClock(observedAt))

			snapshot := collector.Collect(context.Background(), tt.ownerPID)

			if snapshot.Meta.Source != SourceConnections {
				t.Fatalf("expected connection source %q, got %q", SourceConnections, snapshot.Meta.Source)
			}
			if snapshot.Meta.Provider != DefaultConnectionProvider {
				t.Fatalf("expected provider %q, got %q", DefaultConnectionProvider, snapshot.Meta.Provider)
			}
			if snapshot.Meta.Status != StatusConfirmed || !snapshot.Meta.Healthy || !snapshot.Meta.Reachable || !snapshot.Meta.Supported {
				t.Fatalf("expected confirmed healthy metadata, got %+v", snapshot.Meta)
			}
			if !snapshot.Meta.ObservedAt.Equal(observedAt) {
				t.Fatalf("expected observed_at %s, got %s", observedAt, snapshot.Meta.ObservedAt)
			}
			if snapshot.PID != tt.ownerPID {
				t.Fatalf("expected owner pid %d, got %d", tt.ownerPID, snapshot.PID)
			}
			if len(snapshot.Connections) != tt.wantCount {
				t.Fatalf("expected %d owned loopback connections, got %d", tt.wantCount, len(snapshot.Connections))
			}
			if tt.wantCount > 0 && snapshot.Connections[0].RemoteAddress != tt.wantRemote {
				t.Fatalf("expected remote address %q, got %q", tt.wantRemote, snapshot.Connections[0].RemoteAddress)
			}
		})
	}
}

func TestConnectionCollectorCollectReturnsUnavailableOrUnsupportedMetadata(t *testing.T) {
	t.Parallel()

	observedAt := time.Date(2026, time.June, 14, 21, 12, 0, 0, time.UTC)
	tests := []struct {
		name          string
		provider      fakeConnectionProvider
		wantStatus    SnapshotStatus
		wantHealthy   bool
		wantReachable bool
		wantSupported bool
	}{
		{
			name: "provider unavailable",
			provider: fakeConnectionProvider{
				err: errors.New("connections unavailable"),
			},
			wantStatus:    StatusUnavailable,
			wantHealthy:   false,
			wantReachable: false,
			wantSupported: true,
		},
		{
			name: "provider unsupported",
			provider: fakeConnectionProvider{
				err: fmt.Errorf("netstat unsupported: %w", ErrUnsupported),
			},
			wantStatus:    StatusUnsupported,
			wantHealthy:   false,
			wantReachable: false,
			wantSupported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewConnectionCollector(tt.provider, fixedClock(observedAt))

			snapshot := collector.Collect(context.Background(), 4242)

			if snapshot.Meta.Status != tt.wantStatus {
				t.Fatalf("expected status %q, got %q", tt.wantStatus, snapshot.Meta.Status)
			}
			if snapshot.Meta.Healthy != tt.wantHealthy || snapshot.Meta.Reachable != tt.wantReachable || snapshot.Meta.Supported != tt.wantSupported {
				t.Fatalf("unexpected metadata flags: %+v", snapshot.Meta)
			}
			if snapshot.Meta.Error == "" {
				t.Fatal("expected failing provider to surface an error")
			}
			if len(snapshot.Connections) != 0 {
				t.Fatalf("expected no connections on provider failure, got %+v", snapshot.Connections)
			}
		})
	}
}

type fakeConnectionProvider struct {
	connections []ConnectionSample
	err         error
}

func (provider fakeConnectionProvider) Connections(context.Context) ([]ConnectionSample, error) {
	return provider.connections, provider.err
}
