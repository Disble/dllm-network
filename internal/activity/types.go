package activity

import (
	"time"

	"dllm-network/internal/telemetry/ollama"
	"dllm-network/internal/telemetry/orchestrator"
)

type Kind string

const (
	KindInferredModelLoaded  Kind = "inferred-model-loaded"
	KindInferredModelChanged Kind = "inferred-model-changed"
	KindInferredIdle         Kind = "inferred-idle"
	KindInferredUnknown      Kind = "inferred-unknown"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

type Truth string

const (
	TruthInferred Truth = "inferred"
)

type EvidenceKind string

const (
	EvidenceConfirmedRunningModel              EvidenceKind = "confirmed-running-model"
	EvidenceConfirmedProcessAvailable          EvidenceKind = "confirmed-process-available"
	EvidenceConfirmedConnectionActivityPresent EvidenceKind = "confirmed-connection-activity-present"
	EvidenceConfirmedConnectionActivityAbsent  EvidenceKind = "confirmed-connection-activity-absent"
	EvidenceSystemProcessUnavailable           EvidenceKind = "system-process-unavailable"
)

type Evidence struct {
	Kind   EvidenceKind `json:"kind"`
	Detail string       `json:"detail"`
}

type Event struct {
	Kind       Kind       `json:"kind"`
	Truth      Truth      `json:"truth"`
	Model      string     `json:"model,omitempty"`
	Confidence Confidence `json:"confidence"`
	Evidence   []Evidence `json:"evidence"`
	ObservedAt time.Time  `json:"observedAt"`
}

type Input struct {
	Ollama ollama.PollSnapshot
	System orchestrator.SystemSnapshot
}
