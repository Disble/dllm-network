package sqlite

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ollama-telemetry/internal/store"
	"ollama-telemetry/internal/telemetry/inference"
)

// Query implements store.InferenceReader.Query: a filtered list ordered
// most-recent-first (by at DESC), capped at filter.Limit when > 0. Returns
// an empty, non-nil slice (not an error) when nothing matches (spec "No
// matches returns empty, not error").
func (s *Store) Query(ctx context.Context, filter store.Filter) ([]inference.Inference, error) {
	clause, args := whereClause(filter)

	q := fmt.Sprintf(`SELECT id, at, model, endpoint, method, status, status_code,
		streaming, prompt_size, detail FROM inferences%s ORDER BY at DESC, id DESC`, clause)
	if filter.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: query: %w", err)
	}
	defer rows.Close()

	results := make([]inference.Inference, 0)
	for rows.Next() {
		inf, err := scanInference(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: query scan: %w", err)
		}
		results = append(results, inf)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: query rows: %w", err)
	}

	return results, nil
}

func (s *Store) ResolveInferenceContext(ctx context.Context) (store.ResolveInferenceContextResult, error) {
	counts, err := s.resolveCounts(ctx)
	if err != nil {
		return store.ResolveInferenceContextResult{}, err
	}

	models, err := s.resolveFacetCounts(ctx, "model")
	if err != nil {
		return store.ResolveInferenceContextResult{}, err
	}
	endpoints, err := s.resolveFacetCounts(ctx, "endpoint")
	if err != nil {
		return store.ResolveInferenceContextResult{}, err
	}
	statuses, err := s.resolveStatusCounts(ctx)
	if err != nil {
		return store.ResolveInferenceContextResult{}, err
	}
	timeRange, err := s.resolveTimeRange(ctx)
	if err != nil {
		return store.ResolveInferenceContextResult{}, err
	}

	return store.ResolveInferenceContextResult{
		Models:           models,
		Endpoints:        endpoints,
		Statuses:         statuses,
		TimeRange:        timeRange,
		Counts:           counts,
		SupportedFilters: store.SupportedInferenceFilters(),
	}, nil
}

func (s *Store) SearchInferences(ctx context.Context, query store.SearchInferencesQuery) (store.SearchInferencesResult, error) {
	filter := store.Filter{
		Model:    query.Model,
		Endpoint: query.Endpoint,
		Status:   query.Status,
		Since:    query.Since,
		Until:    query.Until,
	}

	clause, args := whereClause(filter)
	cursorClause, cursorArgs, err := searchCursorClause(query)
	if err != nil {
		return store.SearchInferencesResult{}, err
	}
	clause = appendWhereClause(clause, cursorClause)
	args = append(args, cursorArgs...)

	q := fmt.Sprintf(`SELECT id, at, model, endpoint, method, status, status_code,
		streaming, prompt_size FROM inferences%s ORDER BY at DESC, id DESC LIMIT ?`, clause)
	rows, err := s.db.QueryContext(ctx, q, append(args, query.Limit+1)...)
	if err != nil {
		return store.SearchInferencesResult{}, fmt.Errorf("sqlite: search inferences: %w", err)
	}
	defer rows.Close()

	items := make([]store.InferenceSummary, 0, query.Limit+1)
	for rows.Next() {
		item, err := scanInferenceSummary(rows)
		if err != nil {
			return store.SearchInferencesResult{}, fmt.Errorf("sqlite: search inferences scan: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return store.SearchInferencesResult{}, fmt.Errorf("sqlite: search inferences rows: %w", err)
	}

	result := store.SearchInferencesResult{Items: items}
	if len(items) <= query.Limit {
		return result, nil
	}

	visible := append([]store.InferenceSummary(nil), items[:query.Limit]...)
	last := visible[len(visible)-1]
	next, err := encodeSearchCursor(searchCursorPayload{
		Model:    query.Model,
		Endpoint: query.Endpoint,
		Status:   searchCursorStatus(query.Status),
		Since:    query.Since,
		Until:    query.Until,
		At:       last.At,
		ID:       last.ID,
	})
	if err != nil {
		return store.SearchInferencesResult{}, err
	}

	result.Items = visible
	result.NextCursor = next
	return result, nil
}

func (s *Store) GetInferenceContext(ctx context.Context, query store.GetInferenceContextQuery) (store.GetInferenceContextResult, bool, error) {
	_, ok, err := s.Get(ctx, query.ID)
	if err != nil || !ok {
		return store.GetInferenceContextResult{}, ok, err
	}
	return store.GetInferenceContextResult{}, true, nil
}

func (s *Store) resolveCounts(ctx context.Context) (store.InferenceCounts, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM inferences`).Scan(&total); err != nil {
		return store.InferenceCounts{}, fmt.Errorf("sqlite: resolve counts: %w", err)
	}
	return store.InferenceCounts{Total: total}, nil
}

func (s *Store) resolveFacetCounts(ctx context.Context, column string) ([]store.FacetCount, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT %s, COUNT(*) FROM inferences GROUP BY %s ORDER BY COUNT(*) DESC, %s ASC`, column, column, column))
	if err != nil {
		return nil, fmt.Errorf("sqlite: resolve %s: %w", column, err)
	}
	defer rows.Close()

	return scanFacetCounts(rows, column)
}

func (s *Store) resolveStatusCounts(ctx context.Context) ([]store.FacetCount, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM inferences GROUP BY status ORDER BY COUNT(*) DESC, status ASC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: resolve statuses: %w", err)
	}
	defer rows.Close()

	counts := make([]store.FacetCount, 0)
	for rows.Next() {
		var (
			statusValue int
			count       int
		)
		if err := rows.Scan(&statusValue, &count); err != nil {
			return nil, fmt.Errorf("sqlite: resolve statuses scan: %w", err)
		}
		counts = append(counts, store.FacetCount{Value: sqlitePhaseLabel(inference.Phase(statusValue)), Count: count})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: resolve statuses rows: %w", err)
	}
	return counts, nil
}

func (s *Store) resolveTimeRange(ctx context.Context) (store.InferenceTimeRange, error) {
	var oldestNS, latestNS *int64
	if err := s.db.QueryRowContext(ctx, `SELECT MIN(at), MAX(at) FROM inferences`).Scan(&oldestNS, &latestNS); err != nil {
		return store.InferenceTimeRange{}, fmt.Errorf("sqlite: resolve time range: %w", err)
	}
	return store.InferenceTimeRange{Oldest: nanosToTimePtr(oldestNS), Latest: nanosToTimePtr(latestNS)}, nil
}

func scanFacetCounts(rows *sql.Rows, column string) ([]store.FacetCount, error) {
	counts := make([]store.FacetCount, 0)
	for rows.Next() {
		var item store.FacetCount
		if err := rows.Scan(&item.Value, &item.Count); err != nil {
			return nil, fmt.Errorf("sqlite: resolve %s scan: %w", column, err)
		}
		counts = append(counts, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: resolve %s rows: %w", column, err)
	}
	return counts, nil
}

func appendWhereClause(base, extra string) string {
	if extra == "" {
		return base
	}
	if base == "" {
		return " WHERE " + extra
	}
	return base + " AND " + extra
}

type searchCursorPayload struct {
	Model    string    `json:"model,omitempty"`
	Endpoint string    `json:"endpoint,omitempty"`
	Status   string    `json:"status,omitempty"`
	Since    time.Time `json:"since,omitempty"`
	Until    time.Time `json:"until,omitempty"`
	At       time.Time `json:"at"`
	ID       string    `json:"id"`
}

func searchCursorStatus(status *inference.Phase) string {
	if status == nil {
		return ""
	}
	return sqlitePhaseLabel(*status)
}

func encodeSearchCursor(payload searchCursorPayload) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("sqlite: encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeSearchCursor(raw string) (searchCursorPayload, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return searchCursorPayload{}, fmt.Errorf("sqlite: decode cursor: %w", err)
	}
	var payload searchCursorPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return searchCursorPayload{}, fmt.Errorf("sqlite: unmarshal cursor: %w", err)
	}
	if payload.ID == "" || payload.At.IsZero() {
		return searchCursorPayload{}, fmt.Errorf("sqlite: cursor missing position")
	}
	return payload, nil
}

func searchCursorClause(query store.SearchInferencesQuery) (string, []any, error) {
	if query.Cursor == "" {
		return "", nil, nil
	}
	payload, err := decodeSearchCursor(query.Cursor)
	if err != nil {
		return "", nil, err
	}
	if payload.Model != query.Model || payload.Endpoint != query.Endpoint || payload.Status != searchCursorStatus(query.Status) || !payload.Since.Equal(query.Since) || !payload.Until.Equal(query.Until) {
		return "", nil, fmt.Errorf("sqlite: cursor does not match current filters")
	}
	return "(at < ? OR (at = ? AND id < ?))", []any{payload.At.UnixNano(), payload.At.UnixNano(), payload.ID}, nil
}

func scanInferenceSummary(r row) (store.InferenceSummary, error) {
	var (
		id, model, endpoint, method string
		atNanos                     int64
		status, statusCode          int
		streamingInt, promptSize    int
	)
	if err := r.Scan(&id, &atNanos, &model, &endpoint, &method, &status, &statusCode, &streamingInt, &promptSize); err != nil {
		return store.InferenceSummary{}, err
	}
	return store.InferenceSummary{
		ID:         id,
		At:         time.Unix(0, atNanos).UTC(),
		Model:      model,
		Endpoint:   endpoint,
		Method:     method,
		Status:     sqlitePhaseLabel(inference.Phase(status)),
		StatusCode: statusCode,
		Streaming:  streamingInt != 0,
		PromptSize: promptSize,
	}, nil
}

func sqlitePhaseLabel(phase inference.Phase) string {
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

func nanosToTimePtr(value *int64) *time.Time {
	if value == nil {
		return nil
	}
	t := time.Unix(0, *value).UTC()
	return &t
}

// whereClause renders f as a SQL "WHERE ..." clause (or "" when no field is
// set) plus the matching positional args, shared by Query and Stats so the
// filter-to-SQL mapping lives in exactly one place.
func whereClause(f store.Filter) (string, []any) {
	var conds []string
	var args []any

	if f.Model != "" {
		conds = append(conds, "model = ?")
		args = append(args, f.Model)
	}
	if f.Endpoint != "" {
		conds = append(conds, "endpoint = ?")
		args = append(args, f.Endpoint)
	}
	if f.Status != nil {
		conds = append(conds, "status = ?")
		args = append(args, int(*f.Status))
	}
	if !f.Since.IsZero() {
		conds = append(conds, "at >= ?")
		args = append(args, f.Since.UnixNano())
	}
	if !f.Until.IsZero() {
		conds = append(conds, "at < ?")
		args = append(args, f.Until.UnixNano())
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}
