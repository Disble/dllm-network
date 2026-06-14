package system

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestProcessCollectorCollectShapesOllamaProcessSnapshot(t *testing.T) {
	t.Parallel()

	observedAt := time.Date(2026, time.June, 14, 21, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		provider  fakeProcessProvider
		wantFound bool
		wantPID   int32
		wantName  string
	}{
		{
			name: "found ollama process metrics",
			provider: fakeProcessProvider{
				sample: ProcessSample{
					PID:        4242,
					Name:       "ollama.exe",
					CPUPercent: 17.5,
					RSSBytes:   64 * 1024 * 1024,
					ReadBytes:  4096,
					WriteBytes: 2048,
				},
				found: true,
			},
			wantFound: true,
			wantPID:   4242,
			wantName:  "ollama.exe",
		},
		{
			name:      "missing ollama process still reports healthy sample",
			provider:  fakeProcessProvider{},
			wantFound: false,
			wantPID:   0,
			wantName:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewProcessCollector(tt.provider, fixedClock(observedAt))

			snapshot := collector.Collect(context.Background())

			if snapshot.Meta.Source != SourceProcess {
				t.Fatalf("expected process source %q, got %q", SourceProcess, snapshot.Meta.Source)
			}
			if snapshot.Meta.Provider != DefaultProcessProvider {
				t.Fatalf("expected provider %q, got %q", DefaultProcessProvider, snapshot.Meta.Provider)
			}
			if !snapshot.Meta.ObservedAt.Equal(observedAt) {
				t.Fatalf("expected observed_at %s, got %s", observedAt, snapshot.Meta.ObservedAt)
			}
			if snapshot.Meta.Status != StatusConfirmed {
				t.Fatalf("expected confirmed status, got %q", snapshot.Meta.Status)
			}
			if !snapshot.Meta.Healthy || !snapshot.Meta.Reachable || !snapshot.Meta.Supported {
				t.Fatalf("expected healthy reachable supported metadata, got %+v", snapshot.Meta)
			}
			if snapshot.Meta.Error != "" {
				t.Fatalf("expected empty error, got %q", snapshot.Meta.Error)
			}
			if snapshot.Found != tt.wantFound {
				t.Fatalf("expected found=%t, got %t", tt.wantFound, snapshot.Found)
			}
			if snapshot.Process.PID != tt.wantPID {
				t.Fatalf("expected pid %d, got %d", tt.wantPID, snapshot.Process.PID)
			}
			if snapshot.Process.Name != tt.wantName {
				t.Fatalf("expected process name %q, got %q", tt.wantName, snapshot.Process.Name)
			}
		})
	}
}

func TestProcessCollectorCollectReturnsUnavailableOrUnsupportedMetadata(t *testing.T) {
	t.Parallel()

	observedAt := time.Date(2026, time.June, 14, 21, 5, 0, 0, time.UTC)
	tests := []struct {
		name          string
		provider      fakeProcessProvider
		wantStatus    SnapshotStatus
		wantHealthy   bool
		wantReachable bool
		wantSupported bool
	}{
		{
			name: "provider unavailable",
			provider: fakeProcessProvider{
				err: errors.New("process snapshot unavailable"),
			},
			wantStatus:    StatusUnavailable,
			wantHealthy:   false,
			wantReachable: false,
			wantSupported: true,
		},
		{
			name: "provider unsupported",
			provider: fakeProcessProvider{
				err: fmt.Errorf("windows only: %w", ErrUnsupported),
			},
			wantStatus:    StatusUnsupported,
			wantHealthy:   false,
			wantReachable: false,
			wantSupported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewProcessCollector(tt.provider, fixedClock(observedAt))

			snapshot := collector.Collect(context.Background())

			if snapshot.Found {
				t.Fatal("expected failing provider to avoid claiming a process was found")
			}
			if snapshot.Meta.Status != tt.wantStatus {
				t.Fatalf("expected status %q, got %q", tt.wantStatus, snapshot.Meta.Status)
			}
			if snapshot.Meta.Healthy != tt.wantHealthy || snapshot.Meta.Reachable != tt.wantReachable || snapshot.Meta.Supported != tt.wantSupported {
				t.Fatalf("unexpected metadata flags: %+v", snapshot.Meta)
			}
			if snapshot.Meta.Error == "" {
				t.Fatal("expected provider failure to surface an error")
			}
		})
	}
}

type fakeProcessProvider struct {
	sample ProcessSample
	found  bool
	err    error
}

func (provider fakeProcessProvider) LookupOllama(context.Context) (ProcessSample, bool, error) {
	return provider.sample, provider.found, provider.err
}
