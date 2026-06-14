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
			if snapshot.Metrics != tt.metrics {
				t.Fatalf("expected metrics %+v, got %+v", tt.metrics, snapshot.Metrics)
			}
		})
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

			if snapshot.Meta.Status != tt.wantStatus {
				t.Fatalf("expected status %q, got %q", tt.wantStatus, snapshot.Meta.Status)
			}
			if snapshot.Meta.Healthy != tt.wantHealthy || snapshot.Meta.Reachable != tt.wantReachable || snapshot.Meta.Supported != tt.wantSupported {
				t.Fatalf("unexpected metadata flags: %+v", snapshot.Meta)
			}
			if snapshot.Meta.Error == "" {
				t.Fatal("expected failing provider to surface an error")
			}
		})
	}
}

type fakeHostMetricsProvider struct {
	metrics HostMetrics
	err     error
}

func (provider fakeHostMetricsProvider) HostMetrics(context.Context) (HostMetrics, error) {
	return provider.metrics, provider.err
}
