package store

import (
	"time"

	"ollama-telemetry/internal/telemetry/inference"
)

var supportedInferenceFilters = []string{"model", "endpoint", "status", "since", "until"}

// SupportedInferenceFilters returns the declared filter universe for the
// staged MCP discovery/search flow.
func SupportedInferenceFilters() []string {
	return append([]string(nil), supportedInferenceFilters...)
}

type FacetCount struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type InferenceTimeRange struct {
	Oldest *time.Time `json:"oldest,omitempty"`
	Latest *time.Time `json:"latest,omitempty"`
}

type InferenceCounts struct {
	Total int `json:"total"`
}

type ResolveInferenceContextResult struct {
	Models           []FacetCount       `json:"models"`
	Endpoints        []FacetCount       `json:"endpoints"`
	Statuses         []FacetCount       `json:"statuses"`
	TimeRange        InferenceTimeRange `json:"timeRange"`
	Counts           InferenceCounts    `json:"counts"`
	SupportedFilters []string           `json:"supportedFilters"`
}

type SearchInferencesQuery struct {
	Model    string
	Endpoint string
	Status   *inference.Phase
	Since    time.Time
	Until    time.Time
	Limit    int
	Cursor   string
}

type InferenceSummary struct {
	ID         string    `json:"id"`
	At         time.Time `json:"at"`
	Model      string    `json:"model"`
	Endpoint   string    `json:"endpoint"`
	Method     string    `json:"method"`
	Status     string    `json:"status"`
	StatusCode int       `json:"statusCode"`
	Streaming  bool      `json:"streaming"`
	PromptSize int       `json:"promptSize"`
}

type SearchInferencesResult struct {
	Items      []InferenceSummary `json:"items"`
	NextCursor string             `json:"nextCursor,omitempty"`
}

// GetInferenceContextQuery and GetInferenceContextResult reserve the read-side
// contract that PR2 will deepen. PR1 only needs the method shape so the exact
// three-tool MCP surface can compile.
type GetInferenceContextQuery struct {
	ID string
}

type GetInferenceContextResult struct{}
