package mcp

import (
	"fmt"
	"time"

	"dllm-network/internal/store"
	"dllm-network/internal/telemetry/inference"
)

const (
	defaultSearchLimit = 20
	maxSearchLimit     = 100
)

var phaseNames = map[string]inference.Phase{
	"in_progress":   inference.PhaseInProgress,
	"completed":     inference.PhaseCompleted,
	"metadata_only": inference.PhaseMetadataOnly,
	"cancelled":     inference.PhaseCancelled,
}

func parsePhase(s string) (*inference.Phase, error) {
	if s == "" {
		return nil, nil
	}
	p, ok := phaseNames[s]
	if !ok {
		return nil, fmt.Errorf("unknown status %q: must be one of in_progress, completed, metadata_only, cancelled", s)
	}
	return &p, nil
}

func parseTimeFilter(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid RFC3339 timestamp %q: %w", s, err)
	}
	return t, nil
}

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

func copyTimePtr(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	value := src.UTC()
	return &value
}
