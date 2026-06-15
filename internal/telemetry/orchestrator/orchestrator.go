package orchestrator

import (
	"context"
	"sync"
	"time"

	"ollama-telemetry/internal/telemetry"
	"ollama-telemetry/internal/telemetry/system"
)

type State string

const (
	StateRunning State = "running"
	StatePaused  State = "paused"
	StateStopped State = "stopped"
)

type Orchestrator struct {
	mu                  sync.RWMutex
	state               State
	config              telemetry.Config
	processCollector    ProcessCollector
	connectionCollector ConnectionCollector
	hostCollector       HostCollector
	poller              OllamaPoller
	publisher           SnapshotPublisher
	now                 func() time.Time
	newTicker           func(time.Duration) loopTicker
	loopCancel          context.CancelFunc
	loopDone            chan struct{}
}

func New(config telemetry.Config) *Orchestrator {
	return NewWithDependencies(config, Dependencies{})
}

type ProcessCollector interface {
	Collect(context.Context) system.ProcessSnapshot
}

type ConnectionCollector interface {
	Collect(context.Context, int32) system.ConnectionsSnapshot
}

type HostCollector interface {
	Collect(context.Context) system.HostSnapshot
}

type Dependencies struct {
	ProcessCollector    ProcessCollector
	ConnectionCollector ConnectionCollector
	HostCollector       HostCollector
	Poller              OllamaPoller
	Publisher           SnapshotPublisher
	Now                 func() time.Time
	NewTicker           func(time.Duration) loopTicker
}

type SystemSnapshot struct {
	CollectedAt time.Time
	Process     system.ProcessSnapshot
	Connections system.ConnectionsSnapshot
	Host        system.HostSnapshot
}

func NewWithDependencies(config telemetry.Config, deps Dependencies) *Orchestrator {
	processCollector := deps.ProcessCollector
	if processCollector == nil {
		processCollector = system.NewProcessCollector(nil, nil)
	}

	connectionCollector := deps.ConnectionCollector
	if connectionCollector == nil {
		connectionCollector = system.NewConnectionCollector(nil, nil)
	}

	hostCollector := deps.HostCollector
	if hostCollector == nil {
		hostCollector = system.NewHostCollector(nil, nil)
	}

	poller := deps.Poller
	if poller == nil {
		poller = noopOllamaPoller{}
	}

	now := deps.Now
	if now == nil {
		now = time.Now
	}

	newTicker := deps.NewTicker
	if newTicker == nil {
		newTicker = func(interval time.Duration) loopTicker {
			return newTimeTicker(interval)
		}
	}

	return &Orchestrator{
		state:               StateRunning,
		config:              config.WithDefaults(),
		processCollector:    processCollector,
		connectionCollector: connectionCollector,
		hostCollector:       hostCollector,
		poller:              poller,
		publisher:           deps.Publisher,
		now:                 now,
		newTicker:           newTicker,
	}
}

func (orchestrator *Orchestrator) Start(ctx context.Context) error {
	orchestrator.mu.Lock()
	defer orchestrator.mu.Unlock()

	if orchestrator.state == StateStopped || orchestrator.loopDone != nil {
		return nil
	}

	loopCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	ticker := orchestrator.newTicker(smallestCadence(orchestrator.config.Cadence))
	loop := newRuntimeLoop(orchestrator, ticker, done)

	orchestrator.loopCancel = cancel
	orchestrator.loopDone = done

	go loop.run(loopCtx)

	return nil
}

func (orchestrator *Orchestrator) Pause(context.Context) error {
	orchestrator.mu.Lock()
	defer orchestrator.mu.Unlock()

	if orchestrator.state == StateStopped {
		return nil
	}

	orchestrator.state = StatePaused
	return nil
}

func (orchestrator *Orchestrator) Resume(context.Context) error {
	orchestrator.mu.Lock()
	defer orchestrator.mu.Unlock()

	if orchestrator.state == StateStopped {
		return nil
	}

	orchestrator.state = StateRunning
	return nil
}

func (orchestrator *Orchestrator) Stop(ctx context.Context) error {
	orchestrator.mu.Lock()
	orchestrator.state = StateStopped
	cancel := orchestrator.loopCancel
	done := orchestrator.loopDone
	orchestrator.loopCancel = nil
	orchestrator.loopDone = nil
	orchestrator.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if done == nil {
		return nil
	}

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

func (orchestrator *Orchestrator) State() State {
	orchestrator.mu.RLock()
	defer orchestrator.mu.RUnlock()

	return orchestrator.state
}

func (orchestrator *Orchestrator) Config() telemetry.Config {
	orchestrator.mu.RLock()
	defer orchestrator.mu.RUnlock()

	return orchestrator.config
}

func (orchestrator *Orchestrator) CollectSystem(ctx context.Context) SystemSnapshot {
	orchestrator.mu.RLock()
	processCollector := orchestrator.processCollector
	connectionCollector := orchestrator.connectionCollector
	hostCollector := orchestrator.hostCollector
	orchestrator.mu.RUnlock()

	processSnapshot := collectProcess(ctx, processCollector)
	connectionsSnapshot := unsupportedConnections(processSnapshot.Meta.ObservedAt)
	if processSnapshot.Found && processSnapshot.Process.PID > 0 && connectionCollector != nil {
		connectionsSnapshot = connectionCollector.Collect(ctx, processSnapshot.Process.PID)
	}

	hostSnapshot := collectHost(ctx, hostCollector, processSnapshot.Meta.ObservedAt)

	return SystemSnapshot{
		CollectedAt: processSnapshot.Meta.ObservedAt,
		Process:     processSnapshot,
		Connections: connectionsSnapshot,
		Host:        hostSnapshot,
	}
}

func collectProcess(ctx context.Context, collector ProcessCollector) system.ProcessSnapshot {
	return collector.Collect(ctx)
}

func collectHost(ctx context.Context, collector HostCollector, fallback time.Time) system.HostSnapshot {
	if collector == nil {
		return system.HostSnapshot{
			Meta: system.SnapshotMeta{
				Source:     system.SourceHost,
				Provider:   system.DefaultHostProvider,
				ObservedAt: fallback,
				Status:     system.StatusUnsupported,
				Healthy:    false,
				Reachable:  false,
				Supported:  false,
				Error:      "host collector not configured",
			},
		}
	}

	return collector.Collect(ctx)
}

func unsupportedConnections(observedAt time.Time) system.ConnectionsSnapshot {
	if observedAt.IsZero() {
		observedAt = time.Now()
	}

	return system.ConnectionsSnapshot{
		Meta: system.SnapshotMeta{
			Source:     system.SourceConnections,
			Provider:   system.DefaultConnectionProvider,
			ObservedAt: observedAt,
			Status:     system.StatusUnsupported,
			Healthy:    false,
			Reachable:  false,
			Supported:  false,
			Error:      "connection collector requires a confirmed owner pid",
		},
	}
}
