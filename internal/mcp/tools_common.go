package mcp

import (
	"fmt"
	"time"

	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

const (
	defaultSearchLimit = 20
	maxSearchLimit     = 100
)

func parseSearchFilters(model, endpoint, status, since, until string, limit int, cursor string) (store.SearchInferencesQuery, error) {
	phase, err := parsePhase(status)
	if err != nil {
		return store.SearchInferencesQuery{}, err
	}
	parsedSince, err := parseTimeFilter(since)
	if err != nil {
		return store.SearchInferencesQuery{}, err
	}
	parsedUntil, err := parseTimeFilter(until)
	if err != nil {
		return store.SearchInferencesQuery{}, err
	}
	resolvedLimit, err := normalizeSearchLimit(limit)
	if err != nil {
		return store.SearchInferencesQuery{}, err
	}

	return store.SearchInferencesQuery{
		Model:    model,
		Endpoint: endpoint,
		Status:   phase,
		Since:    parsedSince,
		Until:    parsedUntil,
		Limit:    resolvedLimit,
		Cursor:   cursor,
	}, nil
}

func normalizeSearchLimit(limit int) (int, error) {
	if limit <= 0 {
		return defaultSearchLimit, nil
	}
	if limit > maxSearchLimit {
		return 0, fmt.Errorf("limit must be between 1 and %d", maxSearchLimit)
	}
	return limit, nil
}

func phaseLabel(phase inference.Phase) string {
	switch phase {
	case inference.PhaseInProgress:
		return "in_progress"
	case inference.PhaseCompleted:
		return "completed"
	case inference.PhaseMetadataOnly:
		return "metadata_only"
	case inference.PhaseCancelled:
		return "cancelled"
	default:
		return fmt.Sprintf("phase_%d", phase)
	}
}

func copyTimePtr(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	value := src.UTC()
	return &value
}
