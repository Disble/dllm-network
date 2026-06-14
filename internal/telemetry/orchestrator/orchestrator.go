package orchestrator

import (
	"context"
	"sync"

	"ollama-telemetry/internal/telemetry"
)

type State string

const (
	StateRunning State = "running"
	StatePaused  State = "paused"
	StateStopped State = "stopped"
)

type Orchestrator struct {
	mu     sync.RWMutex
	state  State
	config telemetry.Config
}

func New(config telemetry.Config) *Orchestrator {
	return &Orchestrator{
		state:  StateRunning,
		config: config.WithDefaults(),
	}
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

func (orchestrator *Orchestrator) Stop(context.Context) error {
	orchestrator.mu.Lock()
	defer orchestrator.mu.Unlock()

	orchestrator.state = StateStopped
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
