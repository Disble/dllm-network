package activity

import "time"

type Engine struct {
	previousModel string
}

func NewEngine() *Engine {
	return &Engine{}
}

func (engine *Engine) Infer(input Input) Event {
	decision := decide(input, engine.previousModel)
	observedAt := resolveObservedAt(input)

	if decision.model != "" {
		engine.previousModel = decision.model
	}

	return Event{
		Kind:       decision.kind,
		Truth:      TruthInferred,
		Model:      decision.model,
		Confidence: decision.confidence,
		Evidence:   append([]Evidence(nil), decision.evidence...),
		ObservedAt: observedAt,
	}
}

func resolveObservedAt(input Input) time.Time {
	if !input.System.CollectedAt.IsZero() {
		return input.System.CollectedAt
	}
	if !input.Ollama.Running.Meta.ObservedAt.IsZero() {
		return input.Ollama.Running.Meta.ObservedAt
	}
	if !input.Ollama.Meta.ObservedAt.IsZero() {
		return input.Ollama.Meta.ObservedAt
	}

	return time.Time{}
}
