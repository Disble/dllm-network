package app

import (
	"context"

	"ollama-telemetry/internal/activity"
	"ollama-telemetry/internal/dashboard"
	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/orchestrator"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type runtimeEventEmitter func(context.Context, string, ...any)

type wailsEmitter struct {
	emit runtimeEventEmitter
}

func newWailsEmitter() wailsEmitter {
	return wailsEmitter{emit: wruntime.EventsEmit}
}

func (emitter wailsEmitter) Emit(ctx context.Context, event string, payload any) error {
	emitter.emit(ctx, event, payload)
	return nil
}

type activityInferer interface {
	Infer(activity.Input) activity.Event
}

type recentStore interface {
	RecordSnapshot(store.Snapshot)
	AppendActivity(activity.Event)
	Snapshots() []store.Snapshot
	Activities() []activity.Event
}

type snapshotProjector interface {
	Publish(context.Context, dashboard.ProjectionInput) (dashboard.Snapshot, error)
}

type runtimePublisher struct {
	engine    activityInferer
	recent    recentStore
	publisher snapshotProjector
}

func newRuntimePublisherWithDependencies(engine activityInferer, recent recentStore, publisher snapshotProjector) *runtimePublisher {
	return &runtimePublisher{
		engine:    engine,
		recent:    recent,
		publisher: publisher,
	}
}

func (publisher *runtimePublisher) Publish(ctx context.Context, input orchestrator.PublishInput) error {
	confirmedModel := currentConfirmedModel(input)
	if confirmedModel != "" {
		publisher.recent.RecordSnapshot(store.Snapshot{
			ObservedAt:     input.System.CollectedAt,
			ConfirmedModel: confirmedModel,
		})
	}

	activityEvent := publisher.engine.Infer(activity.Input{
		Ollama: input.Ollama,
		System: input.System,
	})
	publisher.recent.AppendActivity(activityEvent)

	_, err := publisher.publisher.Publish(ctx, dashboard.ProjectionInput{
		Ollama:   input.Ollama,
		System:   input.System,
		Activity: activityEvent,
	})
	return err
}

func currentConfirmedModel(input orchestrator.PublishInput) string {
	if len(input.Ollama.Running.Models) == 0 {
		return ""
	}

	model := input.Ollama.Running.Models[0].Name
	if model != "" {
		return model
	}

	return input.Ollama.Running.Models[0].Model
}
