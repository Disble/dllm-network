package orchestrator

import (
	"context"
	"time"

	"ollama-telemetry/internal/telemetry"
	"ollama-telemetry/internal/telemetry/ollama"
)

type OllamaPoller interface {
	Poll(context.Context, ollama.PollRequest) ollama.PollSnapshot
}

type PublishInput struct {
	Ollama ollama.PollSnapshot
	System SystemSnapshot
}

type SnapshotPublisher interface {
	Publish(context.Context, PublishInput) error
}

type loopTicker interface {
	C() <-chan time.Time
	Stop()
}

type runtimeLoop struct {
	orchestrator *Orchestrator
	ticker       loopTicker
	done         chan struct{}
	ollamaAt     time.Time
	systemAt     time.Time
	ollama       ollama.PollSnapshot
	system       SystemSnapshot
}

func newRuntimeLoop(orchestrator *Orchestrator, ticker loopTicker, done chan struct{}) *runtimeLoop {
	return &runtimeLoop{
		orchestrator: orchestrator,
		ticker:       ticker,
		done:         done,
	}
}

func (loop *runtimeLoop) run(ctx context.Context) {
	defer close(loop.done)
	defer loop.ticker.Stop()

	if !loop.cycle(ctx) {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-loop.ticker.C():
			if !loop.cycle(ctx) {
				return
			}
		}
	}
}

func (loop *runtimeLoop) cycle(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}

	state := loop.orchestrator.State()
	if state == StateStopped {
		return false
	}
	if state == StatePaused {
		return true
	}

	config, poller, publisher, now := loop.orchestrator.runtimeLoopDependencies()
	at := now()

	if shouldCollect(loop.ollamaAt, config.Cadence.API, at) {
		loop.ollama = poller.Poll(ctx, ollama.PollRequest{})
		loop.ollamaAt = at
	}

	if shouldCollect(loop.systemAt, config.Cadence.System, at) {
		loop.system = loop.orchestrator.CollectSystem(ctx)
		loop.systemAt = at
	}

	if publisher != nil {
		_ = publisher.Publish(ctx, PublishInput{
			Ollama: loop.ollama,
			System: loop.system,
		})
	}

	return ctx.Err() == nil
}

func (orchestrator *Orchestrator) runtimeLoopDependencies() (telemetry.Config, OllamaPoller, SnapshotPublisher, func() time.Time) {
	orchestrator.mu.RLock()
	defer orchestrator.mu.RUnlock()

	return orchestrator.config, orchestrator.poller, orchestrator.publisher, orchestrator.now
}

func shouldCollect(last time.Time, cadence time.Duration, now time.Time) bool {
	if last.IsZero() {
		return true
	}

	return now.Sub(last) >= cadence
}

func smallestCadence(config telemetry.CadenceConfig) time.Duration {
	interval := config.API
	if interval <= 0 || (config.Logs > 0 && config.Logs < interval) {
		interval = config.Logs
	}
	if interval <= 0 || (config.System > 0 && config.System < interval) {
		interval = config.System
	}
	if interval <= 0 {
		return time.Second
	}

	return interval
}

type noopOllamaPoller struct{}

func (noopOllamaPoller) Poll(context.Context, ollama.PollRequest) ollama.PollSnapshot {
	return ollama.PollSnapshot{}
}

type timeTicker struct {
	ticker *time.Ticker
}

func newTimeTicker(interval time.Duration) loopTicker {
	return timeTicker{ticker: time.NewTicker(interval)}
}

func (ticker timeTicker) C() <-chan time.Time {
	return ticker.ticker.C
}

func (ticker timeTicker) Stop() {
	ticker.ticker.Stop()
}
