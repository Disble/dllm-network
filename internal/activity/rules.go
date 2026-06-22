package activity

import "dllm-network/internal/telemetry/system"

type decision struct {
	kind       Kind
	model      string
	confidence Confidence
	evidence   []Evidence
}

func decide(input Input, previousModel string) decision {
	if input.System.Process.Meta.Status != system.StatusConfirmed || !input.System.Process.Found {
		return decision{
			kind:       KindInferredUnknown,
			confidence: ConfidenceLow,
			evidence: []Evidence{
				{Kind: EvidenceSystemProcessUnavailable, Detail: "process signal is unavailable for passive inference"},
			},
		}
	}

	model := currentModel(input)
	if model == "" {
		return decision{
			kind:       KindInferredUnknown,
			confidence: ConfidenceLow,
			evidence: []Evidence{
				{Kind: EvidenceConfirmedProcessAvailable, Detail: "ollama process is confirmed but no running model is confirmed"},
			},
		}
	}

	if hasConnectionActivity(input) {
		kind := KindInferredModelLoaded
		if previousModel != "" && previousModel != model {
			kind = KindInferredModelChanged
		}

		return decision{
			kind:       kind,
			model:      model,
			confidence: ConfidenceHigh,
			evidence:   runningWithConnectionsEvidence(model),
		}
	}

	return decision{
		kind:       KindInferredIdle,
		model:      model,
		confidence: ConfidenceMedium,
		evidence:   runningWithoutConnectionsEvidence(model),
	}
}

func currentModel(input Input) string {
	if len(input.Ollama.Running.Models) == 0 {
		return ""
	}

	model := input.Ollama.Running.Models[0].Name
	if model != "" {
		return model
	}

	return input.Ollama.Running.Models[0].Model
}

func hasConnectionActivity(input Input) bool {
	return len(input.System.Connections.Connections) > 0
}

func runningWithConnectionsEvidence(model string) []Evidence {
	return []Evidence{
		{Kind: EvidenceConfirmedRunningModel, Detail: "confirmed running model: " + model},
		{Kind: EvidenceConfirmedProcessAvailable, Detail: "ollama process is confirmed and available for passive sampling"},
		{Kind: EvidenceConfirmedConnectionActivityPresent, Detail: "owned loopback connection activity is present"},
	}
}

func runningWithoutConnectionsEvidence(model string) []Evidence {
	return []Evidence{
		{Kind: EvidenceConfirmedRunningModel, Detail: "confirmed running model: " + model},
		{Kind: EvidenceConfirmedProcessAvailable, Detail: "ollama process is confirmed and available for passive sampling"},
		{Kind: EvidenceConfirmedConnectionActivityAbsent, Detail: "no owned loopback connection activity is present"},
	}
}
