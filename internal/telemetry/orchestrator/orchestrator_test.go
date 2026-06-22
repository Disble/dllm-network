package orchestrator

import (
	"context"
	"testing"
	"time"

	"dllm-network/internal/telemetry"
	"dllm-network/internal/telemetry/system"
)

func TestNewUsesDefaultCadencesAndRunningState(t *testing.T) {
	t.Parallel()

	runtime := New(telemetry.Config{})

	if got := runtime.State(); got != StateRunning {
		t.Fatalf("expected new orchestrator to start running, got %q", got)
	}

	config := runtime.Config()
	if config.ShutdownTimeout <= 0 {
		t.Fatalf("expected positive shutdown timeout, got %s", config.ShutdownTimeout)
	}

	if config.Cadence.API <= 0 || config.Cadence.Logs <= 0 || config.Cadence.System <= 0 {
		t.Fatalf("expected positive cadence defaults, got %+v", config.Cadence)
	}
}

func TestOrchestratorPauseResumeStopTransitions(t *testing.T) {
	t.Parallel()

	runtime := New(telemetry.Config{
		ShutdownTimeout: 200 * time.Millisecond,
		Cadence: telemetry.CadenceConfig{
			API:    time.Second,
			Logs:   2 * time.Second,
			System: 3 * time.Second,
		},
	})

	if err := runtime.Pause(context.Background()); err != nil {
		t.Fatalf("pause: %v", err)
	}

	if got := runtime.State(); got != StatePaused {
		t.Fatalf("expected paused state, got %q", got)
	}

	if err := runtime.Resume(context.Background()); err != nil {
		t.Fatalf("resume: %v", err)
	}

	if got := runtime.State(); got != StateRunning {
		t.Fatalf("expected running state after resume, got %q", got)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := runtime.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if got := runtime.State(); got != StateStopped {
		t.Fatalf("expected stopped state after stop, got %q", got)
	}

	if err := runtime.Resume(context.Background()); err != nil {
		t.Fatalf("resume after stop should be ignored safely, got %v", err)
	}

	if got := runtime.State(); got != StateStopped {
		t.Fatalf("expected stopped state to remain terminal, got %q", got)
	}
	if err := runtime.Pause(context.Background()); err != nil {
		t.Fatalf("pause after stop should be ignored safely, got %v", err)
	}
	if got := runtime.State(); got != StateStopped {
		t.Fatalf("expected stopped state to remain terminal after pause, got %q", got)
	}
}

func TestCollectSystemUsesCollectorSeamsAndOwnerPID(t *testing.T) {
	t.Parallel()

	processSnapshot := system.ProcessSnapshot{
		Meta:  system.SnapshotMeta{Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true},
		Found: true,
		Process: system.ProcessSample{
			PID: 4242,
		},
	}
	connectionsSnapshot := system.ConnectionsSnapshot{
		Meta: system.SnapshotMeta{Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true},
		PID:  4242,
	}
	hostSnapshot := system.HostSnapshot{
		Meta: system.SnapshotMeta{Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true},
	}
	processCollector := &stubProcessCollector{snapshot: processSnapshot}
	connectionCollector := &stubConnectionCollector{snapshot: connectionsSnapshot}
	hostCollector := &stubHostCollector{snapshot: hostSnapshot}

	runtime := NewWithDependencies(telemetry.Config{}, Dependencies{
		ProcessCollector:    processCollector,
		ConnectionCollector: connectionCollector,
		HostCollector:       hostCollector,
	})

	snapshot := runtime.CollectSystem(context.Background())

	if processCollector.calls != 1 {
		t.Fatalf("expected process collector to be called once, got %d", processCollector.calls)
	}
	if connectionCollector.calls != 1 {
		t.Fatalf("expected connection collector to be called once, got %d", connectionCollector.calls)
	}
	if connectionCollector.lastPID != 4242 {
		t.Fatalf("expected connection collector to receive pid 4242, got %d", connectionCollector.lastPID)
	}
	if hostCollector.calls != 1 {
		t.Fatalf("expected host collector to be called once, got %d", hostCollector.calls)
	}
	if snapshot.Process.Process.PID != 4242 {
		t.Fatalf("expected snapshot to include process pid 4242, got %d", snapshot.Process.Process.PID)
	}
	if snapshot.Connections.PID != 4242 {
		t.Fatalf("expected snapshot to include connections pid 4242, got %d", snapshot.Connections.PID)
	}
	if snapshot.Host.Meta.Status != system.StatusConfirmed {
		t.Fatalf("expected host status confirmed, got %q", snapshot.Host.Meta.Status)
	}
}

func TestCollectSystemSkipsConnectionOwnershipWhenProcessIsUnavailable(t *testing.T) {
	t.Parallel()

	processCollector := &stubProcessCollector{
		snapshot: system.ProcessSnapshot{
			Meta: system.SnapshotMeta{Status: system.StatusUnavailable, Healthy: false, Reachable: false, Supported: true},
		},
	}
	connectionCollector := &stubConnectionCollector{
		snapshot: system.ConnectionsSnapshot{
			Meta: system.SnapshotMeta{Status: system.StatusUnsupported, Healthy: false, Reachable: false, Supported: false},
		},
	}
	hostCollector := &stubHostCollector{
		snapshot: system.HostSnapshot{
			Meta: system.SnapshotMeta{Status: system.StatusConfirmed, Healthy: true, Reachable: true, Supported: true},
		},
	}

	runtime := NewWithDependencies(telemetry.Config{}, Dependencies{
		ProcessCollector:    processCollector,
		ConnectionCollector: connectionCollector,
		HostCollector:       hostCollector,
	})

	snapshot := runtime.CollectSystem(context.Background())

	if connectionCollector.calls != 0 {
		t.Fatalf("expected connection collector to be skipped when no owner pid is confirmed, got %d calls", connectionCollector.calls)
	}
	if snapshot.Connections.Meta.Status != system.StatusUnsupported {
		t.Fatalf("expected skipped connections snapshot to be unsupported, got %q", snapshot.Connections.Meta.Status)
	}
	if snapshot.Connections.Meta.Supported {
		t.Fatalf("expected skipped connections snapshot to report unsupported metadata, got %+v", snapshot.Connections.Meta)
	}
	if snapshot.Host.Meta.Status != system.StatusConfirmed {
		t.Fatalf("expected host collection to continue, got %q", snapshot.Host.Meta.Status)
	}
}

type stubProcessCollector struct {
	snapshot system.ProcessSnapshot
	calls    int
}

func (collector *stubProcessCollector) Collect(context.Context) system.ProcessSnapshot {
	collector.calls++
	return collector.snapshot
}

type stubConnectionCollector struct {
	snapshot system.ConnectionsSnapshot
	calls    int
	lastPID  int32
}

func (collector *stubConnectionCollector) Collect(_ context.Context, pid int32) system.ConnectionsSnapshot {
	collector.calls++
	collector.lastPID = pid
	return collector.snapshot
}

type stubHostCollector struct {
	snapshot system.HostSnapshot
	calls    int
}

func (collector *stubHostCollector) Collect(context.Context) system.HostSnapshot {
	collector.calls++
	return collector.snapshot
}
