package sqlite

import (
	"time"

	"dllm-network/internal/telemetry/inference"
)

// row is satisfied by both *sql.Row (Get) and *sql.Rows (Query, slice 3) so
// scanInference can be shared by both call sites without duplicating the
// column list or the detail-unmarshal step.
type row interface {
	Scan(dest ...any) error
}

// scanInference reads the scalar columns selected by selectByIDSQL (and,
// later, the query API's SELECT) plus the detail JSON blob, and assembles a
// fully-populated inference.Inference. Token-stat scalar columns are NOT
// read here in slice 1 — Get/Query rebuild Tokens entirely from the detail
// blob via unmarshalDetail, which already carries the full TokenStats.
func scanInference(r row) (inference.Inference, error) {
	var (
		id, model, endpoint, method, detailJSON string
		atNanos                                 int64
		status, statusCode, promptSize          int
		streamingInt                            int
	)

	if err := r.Scan(&id, &atNanos, &model, &endpoint, &method, &status,
		&statusCode, &streamingInt, &promptSize, &detailJSON); err != nil {
		return inference.Inference{}, err
	}

	inf := inference.Inference{
		ID:         id,
		At:         time.Unix(0, atNanos).UTC(),
		Model:      model,
		Endpoint:   endpoint,
		Method:     method,
		Status:     inference.Phase(status),
		StatusCode: statusCode,
		Streaming:  streamingInt != 0,
		PromptSize: promptSize,
	}

	if err := unmarshalDetail(detailJSON, &inf); err != nil {
		return inference.Inference{}, err
	}

	return inf, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nullableInt/nullableInt64/nullableFloat wrap a derived scalar so it is
// written as SQL NULL when present=false. present is always
// (TokenStats != nil) at the call site — this is what preserves the domain's
// nil-vs-zero "not applicable" contract in the flattened token-stat columns,
// per design D2.
func nullableInt(present bool, v int) any {
	if !present {
		return nil
	}
	return v
}

func nullableInt64(present bool, v int64) any {
	if !present {
		return nil
	}
	return v
}

func nullableFloat(present bool, v float64) any {
	if !present {
		return nil
	}
	return v
}

func tokensPromptEvalCount(t *inference.TokenStats) int {
	if t == nil {
		return 0
	}
	return t.PromptEvalCount
}

func tokensEvalCount(t *inference.TokenStats) int {
	if t == nil {
		return 0
	}
	return t.EvalCount
}

func tokensEvalDurationNS(t *inference.TokenStats) int64 {
	if t == nil {
		return 0
	}
	return t.EvalDuration.Nanoseconds()
}

func tokensTotalDurationNS(t *inference.TokenStats) int64 {
	if t == nil {
		return 0
	}
	return t.TotalDuration.Nanoseconds()
}

func tokensLoadDurationNS(t *inference.TokenStats) int64 {
	if t == nil {
		return 0
	}
	return t.LoadDuration.Nanoseconds()
}

func tokensPerSec(t *inference.TokenStats) float64 {
	if t == nil {
		return 0
	}
	return t.PerSec
}

func tokensLatencyMS(t *inference.TokenStats) float64 {
	if t == nil {
		return 0
	}
	return t.LatencyMS
}
