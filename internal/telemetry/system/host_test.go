package system

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestHostCollectorCollectShapesHostMetricsSnapshot(t *testing.T) {
	t.Parallel()

	observedAt := time.Date(2026, time.June, 14, 21, 15, 0, 0, time.UTC)
	tests := []struct {
		name    string
		metrics HostMetrics
	}{
		{
			name: "cpu memory and swap metrics",
			metrics: HostMetrics{
				CPUPercent:       38.5,
				MemoryUsedBytes:  8 * 1024 * 1024 * 1024,
				MemoryTotalBytes: 16 * 1024 * 1024 * 1024,
				SwapUsedBytes:    2 * 1024 * 1024 * 1024,
				SwapTotalBytes:   4 * 1024 * 1024 * 1024,
			},
		},
		{
			name: "zero swap remains an honest sample",
			metrics: HostMetrics{
				CPUPercent:       5,
				MemoryUsedBytes:  512,
				MemoryTotalBytes: 1024,
				SwapUsedBytes:    0,
				SwapTotalBytes:   0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewHostCollector(fakeHostMetricsProvider{metrics: tt.metrics}, fixedClock(observedAt))

			snapshot := collector.Collect(context.Background())

			assertConfirmedHostSnapshot(t, snapshot, observedAt, tt.metrics)
		})
	}
}

// assertConfirmedHostSnapshot verifies a successful host collection exposes the
// expected source, provider, metadata flags, observed time, and metrics values.
func assertConfirmedHostSnapshot(t *testing.T, snapshot HostSnapshot, observedAt time.Time, wantMetrics HostMetrics) {
	t.Helper()
	if snapshot.Meta.Source != SourceHost {
		t.Fatalf("expected host source %q, got %q", SourceHost, snapshot.Meta.Source)
	}
	if snapshot.Meta.Provider != DefaultHostProvider {
		t.Fatalf("expected provider %q, got %q", DefaultHostProvider, snapshot.Meta.Provider)
	}
	if snapshot.Meta.Status != StatusConfirmed || !snapshot.Meta.Healthy || !snapshot.Meta.Reachable || !snapshot.Meta.Supported {
		t.Fatalf("expected confirmed healthy metadata, got %+v", snapshot.Meta)
	}
	if !snapshot.Meta.ObservedAt.Equal(observedAt) {
		t.Fatalf("expected observed_at %s, got %s", observedAt, snapshot.Meta.ObservedAt)
	}
	if snapshot.Metrics != wantMetrics {
		t.Fatalf("expected metrics %+v, got %+v", wantMetrics, snapshot.Metrics)
	}
}

func TestHostCollectorCollectReturnsUnavailableOrUnsupportedMetadata(t *testing.T) {
	t.Parallel()

	observedAt := time.Date(2026, time.June, 14, 21, 17, 0, 0, time.UTC)
	tests := []struct {
		name          string
		provider      fakeHostMetricsProvider
		wantStatus    SnapshotStatus
		wantHealthy   bool
		wantReachable bool
		wantSupported bool
	}{
		{
			name: "provider unavailable",
			provider: fakeHostMetricsProvider{
				err: errors.New("host metrics unavailable"),
			},
			wantStatus:    StatusUnavailable,
			wantHealthy:   false,
			wantReachable: false,
			wantSupported: true,
		},
		{
			name: "provider unsupported",
			provider: fakeHostMetricsProvider{
				err: fmt.Errorf("host metrics unsupported: %w", ErrUnsupported),
			},
			wantStatus:    StatusUnsupported,
			wantHealthy:   false,
			wantReachable: false,
			wantSupported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewHostCollector(tt.provider, fixedClock(observedAt))

			snapshot := collector.Collect(context.Background())

			assertFailingHostSnapshot(t, snapshot, tt.wantStatus, tt.wantHealthy, tt.wantReachable, tt.wantSupported)
		})
	}
}

// assertFailingHostSnapshot verifies an unavailable/unsupported host provider
// reports the expected metadata flags and a non-empty error.
func assertFailingHostSnapshot(t *testing.T, snapshot HostSnapshot, wantStatus SnapshotStatus, wantHealthy, wantReachable, wantSupported bool) {
	t.Helper()
	if snapshot.Meta.Status != wantStatus {
		t.Fatalf("expected status %q, got %q", wantStatus, snapshot.Meta.Status)
	}
	if snapshot.Meta.Healthy != wantHealthy || snapshot.Meta.Reachable != wantReachable || snapshot.Meta.Supported != wantSupported {
		t.Fatalf("unexpected metadata flags: %+v", snapshot.Meta)
	}
	if snapshot.Meta.Error == "" {
		t.Fatal("expected failing provider to surface an error")
	}
}

type fakeHostMetricsProvider struct {
	metrics HostMetrics
	err     error
}

func (provider fakeHostMetricsProvider) HostMetrics(context.Context) (HostMetrics, error) {
	return provider.metrics, provider.err
}
