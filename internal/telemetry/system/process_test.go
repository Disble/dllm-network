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

			assertConfirmedProcessSnapshot(t, snapshot, observedAt, tt.wantFound, tt.wantPID, tt.wantName)
		})
	}
}

// assertConfirmedProcessSnapshot verifies a successful process collection
// exposes the expected source, provider, metadata flags, observed time, found
// flag, and process identifiers.
func assertConfirmedProcessSnapshot(t *testing.T, snapshot ProcessSnapshot, observedAt time.Time, wantFound bool, wantPID int32, wantName string) {
	t.Helper()
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
	if snapshot.Found != wantFound {
		t.Fatalf("expected found=%t, got %t", wantFound, snapshot.Found)
	}
	if snapshot.Process.PID != wantPID {
		t.Fatalf("expected pid %d, got %d", wantPID, snapshot.Process.PID)
	}
	if snapshot.Process.Name != wantName {
		t.Fatalf("expected process name %q, got %q", wantName, snapshot.Process.Name)
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

			assertFailingProcessSnapshot(t, snapshot, tt.wantStatus, tt.wantHealthy, tt.wantReachable, tt.wantSupported)
		})
	}
}

// assertFailingProcessSnapshot verifies an unavailable/unsupported process
// provider does not claim a process was found and reports the expected metadata
// flags plus a non-empty error.
func assertFailingProcessSnapshot(t *testing.T, snapshot ProcessSnapshot, wantStatus SnapshotStatus, wantHealthy, wantReachable, wantSupported bool) {
	t.Helper()
	if snapshot.Found {
		t.Fatal("expected failing provider to avoid claiming a process was found")
	}
	if snapshot.Meta.Status != wantStatus {
		t.Fatalf("expected status %q, got %q", wantStatus, snapshot.Meta.Status)
	}
	if snapshot.Meta.Healthy != wantHealthy || snapshot.Meta.Reachable != wantReachable || snapshot.Meta.Supported != wantSupported {
		t.Fatalf("unexpected metadata flags: %+v", snapshot.Meta)
	}
	if snapshot.Meta.Error == "" {
		t.Fatal("expected provider failure to surface an error")
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
