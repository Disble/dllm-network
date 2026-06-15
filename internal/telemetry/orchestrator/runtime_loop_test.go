package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"ollama-telemetry/internal/telemetry"
	"ollama-telemetry/internal/telemetry/ollama"
	"ollama-telemetry/internal/telemetry/system"
)

func TestStartRunsImmediateCycleAndPublishesCurrentSnapshots(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.June, 14, 12, 0, 0, 0, time.UTC)
	clock := newFakeClock(base)
	ticker := newFakeTicker()
	poller := &fakePoller{snapshots: []ollama.PollSnapshot{confirmedPollSnapshot(base, "model-1")}}
	publisher := newFakePublisher(nil)
	runtime := newLoopTestOrchestrator(clock, ticker, poller, publisher, loopTestSequences{
		processes:   []system.ProcessSnapshot{confirmedProcessSnapshot(base, 1001)},
		connections: []system.ConnectionsSnapshot{confirmedConnectionsSnapshot(base, 1001)},
		hosts:       []system.HostSnapshot{confirmedHostSnapshot(base)},
	})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { stopLoopTestRuntime(t, runtime) })

	publisher.WaitForCalls(t, 1)

	if poller.Calls() != 1 {
		t.Fatalf("expected immediate ollama poll, got %d calls", poller.Calls())
	}
	if got := publisher.Inputs(); len(got) != 1 {
		t.Fatalf("expected one publish call, got %d", len(got))
	} else {
		if model := got[0].Ollama.Running.Models[0].Name; model != "model-1" {
			t.Fatalf("expected published running model model-1, got %q", model)
		}
		if pid := got[0].System.Process.Process.PID; pid != 1001 {
			t.Fatalf("expected published process pid 1001, got %d", pid)
		}
	}
}

func TestStartGatesCollectorsByCadenceAndPublishesCachedSnapshots(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.June, 14, 12, 0, 0, 0, time.UTC)
	clock := newFakeClock(base)
	ticker := newFakeTicker()
	poller := &fakePoller{snapshots: []ollama.PollSnapshot{
		confirmedPollSnapshot(base, "model-1"),
		confirmedPollSnapshot(base.Add(5*time.Second), "model-2"),
	}}
	publisher := newFakePublisher(nil)
	runtime := newLoopTestOrchestratorWithConfig(clock, ticker, poller, publisher, loopTestSequences{
		processes: []system.ProcessSnapshot{
			confirmedProcessSnapshot(base, 1001),
			confirmedProcessSnapshot(base.Add(3*time.Second), 1002),
		},
		connections: []system.ConnectionsSnapshot{
			confirmedConnectionsSnapshot(base, 1001),
			confirmedConnectionsSnapshot(base.Add(3*time.Second), 1002),
		},
		hosts: []system.HostSnapshot{
			confirmedHostSnapshot(base),
			confirmedHostSnapshot(base.Add(3 * time.Second)),
		},
	}, telemetry.Config{
		ShutdownTimeout: time.Second,
		Cadence: telemetry.CadenceConfig{
			API:    5 * time.Second,
			Logs:   5 * time.Second,
			System: 3 * time.Second,
		},
	})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { stopLoopTestRuntime(t, runtime) })

	publisher.WaitForCalls(t, 1)

	clock.Advance(3 * time.Second)
	ticker.Tick(clock.Now())
	publisher.WaitForCalls(t, 2)

	if poller.Calls() != 1 {
		t.Fatalf("expected ollama poll to stay cached at system cadence, got %d calls", poller.Calls())
	}
	secondPublish := publisher.Inputs()[1]
	if model := secondPublish.Ollama.Running.Models[0].Name; model != "model-1" {
		t.Fatalf("expected cached ollama model model-1 on system-only tick, got %q", model)
	}
	if pid := secondPublish.System.Process.Process.PID; pid != 1002 {
		t.Fatalf("expected refreshed system pid 1002 on system-only tick, got %d", pid)
	}

	clock.Advance(2 * time.Second)
	ticker.Tick(clock.Now())
	publisher.WaitForCalls(t, 3)

	if poller.Calls() != 2 {
		t.Fatalf("expected ollama poll after api cadence elapsed, got %d calls", poller.Calls())
	}
	thirdPublish := publisher.Inputs()[2]
	if model := thirdPublish.Ollama.Running.Models[0].Name; model != "model-2" {
		t.Fatalf("expected refreshed ollama model model-2 on api tick, got %q", model)
	}
	if pid := thirdPublish.System.Process.Process.PID; pid != 1002 {
		t.Fatalf("expected cached system pid 1002 on api-only tick, got %d", pid)
	}
}

func TestPauseSkipsCollectionUntilResume(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.June, 14, 12, 0, 0, 0, time.UTC)
	clock := newFakeClock(base)
	ticker := newFakeTicker()
	poller := &fakePoller{snapshots: []ollama.PollSnapshot{
		confirmedPollSnapshot(base, "model-1"),
		confirmedPollSnapshot(base.Add(6*time.Second), "model-2"),
	}}
	publisher := newFakePublisher(nil)
	runtime := newLoopTestOrchestratorWithConfig(clock, ticker, poller, publisher, loopTestSequences{
		processes: []system.ProcessSnapshot{
			confirmedProcessSnapshot(base, 1001),
			confirmedProcessSnapshot(base.Add(6*time.Second), 1002),
		},
		connections: []system.ConnectionsSnapshot{
			confirmedConnectionsSnapshot(base, 1001),
			confirmedConnectionsSnapshot(base.Add(6*time.Second), 1002),
		},
		hosts: []system.HostSnapshot{
			confirmedHostSnapshot(base),
			confirmedHostSnapshot(base.Add(6 * time.Second)),
		},
	}, telemetry.Config{
		ShutdownTimeout: time.Second,
		Cadence: telemetry.CadenceConfig{
			API:    3 * time.Second,
			Logs:   3 * time.Second,
			System: 3 * time.Second,
		},
	})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { stopLoopTestRuntime(t, runtime) })

	publisher.WaitForCalls(t, 1)

	if err := runtime.Pause(context.Background()); err != nil {
		t.Fatalf("pause: %v", err)
	}

	clock.Advance(3 * time.Second)
	ticker.Tick(clock.Now())
	publisher.AssertCallsStayAt(t, 1, 150*time.Millisecond)

	if poller.Calls() != 1 {
		t.Fatalf("expected paused loop to skip polling, got %d calls", poller.Calls())
	}

	if err := runtime.Resume(context.Background()); err != nil {
		t.Fatalf("resume: %v", err)
	}

	clock.Advance(3 * time.Second)
	ticker.Tick(clock.Now())
	publisher.WaitForCalls(t, 2)

	if poller.Calls() != 2 {
		t.Fatalf("expected resumed loop to poll again, got %d calls", poller.Calls())
	}
}

func TestStopCancelsInFlightCycleAndWaitsForLoopExit(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.June, 14, 12, 0, 0, 0, time.UTC)
	clock := newFakeClock(base)
	ticker := newFakeTicker()
	poller := &fakePoller{snapshots: []ollama.PollSnapshot{confirmedPollSnapshot(base, "model-1")}}
	publisher := newBlockingPublisher()
	runtime := newLoopTestOrchestrator(clock, ticker, poller, publisher, loopTestSequences{
		processes:   []system.ProcessSnapshot{confirmedProcessSnapshot(base, 1001)},
		connections: []system.ConnectionsSnapshot{confirmedConnectionsSnapshot(base, 1001)},
		hosts:       []system.HostSnapshot{confirmedHostSnapshot(base)},
	})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}

	publisher.WaitForEntry(t)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := runtime.Stop(stopCtx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if got := runtime.State(); got != StateStopped {
		t.Fatalf("expected stopped state after stop, got %q", got)
	}
	if !ticker.Stopped() {
		t.Fatal("expected stop to stop the runtime ticker")
	}
}

func TestPublisherErrorDoesNotStopRuntimeLoop(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.June, 14, 12, 0, 0, 0, time.UTC)
	clock := newFakeClock(base)
	ticker := newFakeTicker()
	poller := &fakePoller{snapshots: []ollama.PollSnapshot{
		confirmedPollSnapshot(base, "model-1"),
		confirmedPollSnapshot(base.Add(3*time.Second), "model-2"),
	}}
	publisher := newFakePublisher([]error{errors.New("emit failed"), nil})
	runtime := newLoopTestOrchestratorWithConfig(clock, ticker, poller, publisher, loopTestSequences{
		processes: []system.ProcessSnapshot{
			confirmedProcessSnapshot(base, 1001),
			confirmedProcessSnapshot(base.Add(3*time.Second), 1002),
		},
		connections: []system.ConnectionsSnapshot{
			confirmedConnectionsSnapshot(base, 1001),
			confirmedConnectionsSnapshot(base.Add(3*time.Second), 1002),
		},
		hosts: []system.HostSnapshot{
			confirmedHostSnapshot(base),
			confirmedHostSnapshot(base.Add(3 * time.Second)),
		},
	}, telemetry.Config{
		ShutdownTimeout: time.Second,
		Cadence: telemetry.CadenceConfig{
			API:    3 * time.Second,
			Logs:   3 * time.Second,
			System: 3 * time.Second,
		},
	})

	if err := runtime.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { stopLoopTestRuntime(t, runtime) })

	publisher.WaitForCalls(t, 1)

	clock.Advance(3 * time.Second)
	ticker.Tick(clock.Now())
	publisher.WaitForCalls(t, 2)

	if poller.Calls() != 2 {
		t.Fatalf("expected runtime loop to continue polling after publish error, got %d calls", poller.Calls())
	}
	if got := runtime.State(); got != StateRunning {
		t.Fatalf("expected runtime loop to remain running after publish error, got %q", got)
	}
}

type loopTestSequences struct {
	processes   []system.ProcessSnapshot
	connections []system.ConnectionsSnapshot
	hosts       []system.HostSnapshot
}

func newLoopTestOrchestrator(clock *fakeClock, ticker *fakeTicker, poller *fakePoller, publisher SnapshotPublisher, sequences loopTestSequences) *Orchestrator {
	return newLoopTestOrchestratorWithConfig(clock, ticker, poller, publisher, sequences, telemetry.Config{
		ShutdownTimeout: time.Second,
		Cadence: telemetry.CadenceConfig{
			API:    5 * time.Second,
			Logs:   5 * time.Second,
			System: 3 * time.Second,
		},
	})
}

func newLoopTestOrchestratorWithConfig(clock *fakeClock, ticker *fakeTicker, poller *fakePoller, publisher SnapshotPublisher, sequences loopTestSequences, config telemetry.Config) *Orchestrator {
	return NewWithDependencies(config, Dependencies{
		ProcessCollector:    &sequenceProcessCollector{snapshots: sequences.processes},
		ConnectionCollector: &sequenceConnectionCollector{snapshots: sequences.connections},
		HostCollector:       &sequenceHostCollector{snapshots: sequences.hosts},
		Poller:              poller,
		Publisher:           publisher,
		Now:                 clock.Now,
		NewTicker: func(time.Duration) loopTicker {
			return ticker
		},
	})
}

func stopLoopTestRuntime(t *testing.T, runtime *Orchestrator) {
	t.Helper()
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runtime.Stop(stopCtx); err != nil {
		t.Fatalf("stop cleanup: %v", err)
	}
}

func confirmedPollSnapshot(observedAt time.Time, model string) ollama.PollSnapshot {
	return ollama.PollSnapshot{
		Meta: ollama.SnapshotMeta{
			ObservedAt: observedAt,
			Status:     ollama.StatusConfirmed,
			Reachable:  true,
		},
		Running: ollama.RunningModelsSnapshot{
			Meta: ollama.SnapshotMeta{
				ObservedAt:      observedAt,
				LastConfirmedAt: observedAt,
				Status:          ollama.StatusConfirmed,
				Reachable:       true,
			},
			Models: []ollama.RunningModel{{Name: model, Model: model}},
		},
	}
}

func confirmedProcessSnapshot(observedAt time.Time, pid int32) system.ProcessSnapshot {
	return system.ProcessSnapshot{
		Meta: system.SnapshotMeta{
			ObservedAt: observedAt,
			Status:     system.StatusConfirmed,
			Healthy:    true,
			Reachable:  true,
			Supported:  true,
		},
		Found: true,
		Process: system.ProcessSample{
			PID: pid,
		},
	}
}

func confirmedConnectionsSnapshot(observedAt time.Time, pid int32) system.ConnectionsSnapshot {
	return system.ConnectionsSnapshot{
		Meta: system.SnapshotMeta{
			ObservedAt: observedAt,
			Status:     system.StatusConfirmed,
			Healthy:    true,
			Reachable:  true,
			Supported:  true,
		},
		PID: pid,
	}
}

func confirmedHostSnapshot(observedAt time.Time) system.HostSnapshot {
	return system.HostSnapshot{
		Meta: system.SnapshotMeta{
			ObservedAt: observedAt,
			Status:     system.StatusConfirmed,
			Healthy:    true,
			Reachable:  true,
			Supported:  true,
		},
	}
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(now time.Time) *fakeClock {
	return &fakeClock{now: now}
}

func (clock *fakeClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *fakeClock) Advance(duration time.Duration) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.now = clock.now.Add(duration)
}

type fakeTicker struct {
	channel chan time.Time
	mu      sync.Mutex
	stopped bool
}

func newFakeTicker() *fakeTicker {
	return &fakeTicker{channel: make(chan time.Time, 8)}
}

func (ticker *fakeTicker) C() <-chan time.Time {
	return ticker.channel
}

func (ticker *fakeTicker) Stop() {
	ticker.mu.Lock()
	defer ticker.mu.Unlock()
	ticker.stopped = true
}

func (ticker *fakeTicker) Tick(at time.Time) {
	ticker.channel <- at
}

func (ticker *fakeTicker) Stopped() bool {
	ticker.mu.Lock()
	defer ticker.mu.Unlock()
	return ticker.stopped
}

type fakePoller struct {
	mu        sync.Mutex
	snapshots []ollama.PollSnapshot
	calls     int
}

func (poller *fakePoller) Poll(context.Context, ollama.PollRequest) ollama.PollSnapshot {
	poller.mu.Lock()
	defer poller.mu.Unlock()
	index := poller.calls
	poller.calls++
	if len(poller.snapshots) == 0 {
		return ollama.PollSnapshot{}
	}
	if index >= len(poller.snapshots) {
		return poller.snapshots[len(poller.snapshots)-1]
	}
	return poller.snapshots[index]
}

func (poller *fakePoller) Calls() int {
	poller.mu.Lock()
	defer poller.mu.Unlock()
	return poller.calls
}

type fakePublisher struct {
	mu       sync.Mutex
	inputs   []PublishInput
	errs     []error
	calledCh chan struct{}
}

func newFakePublisher(errs []error) *fakePublisher {
	return &fakePublisher{
		errs:     errs,
		calledCh: make(chan struct{}, 16),
	}
}

func (publisher *fakePublisher) Publish(_ context.Context, input PublishInput) error {
	publisher.mu.Lock()
	callIndex := len(publisher.inputs)
	publisher.inputs = append(publisher.inputs, input)
	var err error
	if callIndex < len(publisher.errs) {
		err = publisher.errs[callIndex]
	}
	publisher.mu.Unlock()
	publisher.calledCh <- struct{}{}
	return err
}

func (publisher *fakePublisher) WaitForCalls(t *testing.T, expected int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for len(publisher.Inputs()) < expected {
		select {
		case <-publisher.calledCh:
		case <-deadline:
			t.Fatalf("timed out waiting for %d publisher calls, got %d", expected, len(publisher.Inputs()))
		}
	}
}

func (publisher *fakePublisher) AssertCallsStayAt(t *testing.T, expected int, duration time.Duration) {
	t.Helper()
	if got := len(publisher.Inputs()); got != expected {
		t.Fatalf("expected %d publisher calls before stability check, got %d", expected, got)
	}
	select {
	case <-publisher.calledCh:
		t.Fatalf("expected publisher calls to stay at %d during %s, got %d", expected, duration, len(publisher.Inputs()))
	case <-time.After(duration):
	}
	if got := len(publisher.Inputs()); got != expected {
		t.Fatalf("expected publisher calls to remain at %d, got %d", expected, got)
	}
}

func (publisher *fakePublisher) Inputs() []PublishInput {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	return append([]PublishInput(nil), publisher.inputs...)
}

type blockingPublisher struct {
	entered chan struct{}
	once    sync.Once
}

func newBlockingPublisher() *blockingPublisher {
	return &blockingPublisher{entered: make(chan struct{})}
}

func (publisher *blockingPublisher) Publish(ctx context.Context, _ PublishInput) error {
	publisher.once.Do(func() {
		close(publisher.entered)
	})
	<-ctx.Done()
	return ctx.Err()
}

func (publisher *blockingPublisher) WaitForEntry(t *testing.T) {
	t.Helper()
	select {
	case <-publisher.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for blocking publish")
	}
}

type sequenceProcessCollector struct {
	mu        sync.Mutex
	snapshots []system.ProcessSnapshot
	calls     int
}

func (collector *sequenceProcessCollector) Collect(context.Context) system.ProcessSnapshot {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	index := collector.calls
	collector.calls++
	if index >= len(collector.snapshots) {
		return collector.snapshots[len(collector.snapshots)-1]
	}
	return collector.snapshots[index]
}

type sequenceConnectionCollector struct {
	mu        sync.Mutex
	snapshots []system.ConnectionsSnapshot
	calls     int
}

func (collector *sequenceConnectionCollector) Collect(context.Context, int32) system.ConnectionsSnapshot {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	index := collector.calls
	collector.calls++
	if index >= len(collector.snapshots) {
		return collector.snapshots[len(collector.snapshots)-1]
	}
	return collector.snapshots[index]
}

type sequenceHostCollector struct {
	mu        sync.Mutex
	snapshots []system.HostSnapshot
	calls     int
}

func (collector *sequenceHostCollector) Collect(context.Context) system.HostSnapshot {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	index := collector.calls
	collector.calls++
	if index >= len(collector.snapshots) {
		return collector.snapshots[len(collector.snapshots)-1]
	}
	return collector.snapshots[index]
}
