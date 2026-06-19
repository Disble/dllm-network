package store

import (
	"fmt"
	"time"

	"ollama-telemetry/internal/telemetry/inference"
)

var supportedInferenceFilters = []string{"model", "endpoint", "status", "since", "until"}

const defaultInferenceContextBodyLimit = 4096

// SupportedInferenceFilters returns the declared filter universe for the
// staged MCP discovery/search flow.
func SupportedInferenceFilters() []string {
	return append([]string(nil), supportedInferenceFilters...)
}

// InferenceStatusLabel converts the domain phase enum into the stable wire
// labels used by the staged MCP contract and read-side DTOs.
func InferenceStatusLabel(phase inference.Phase) string {
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

type InferenceContextSection string

const (
	InferenceContextSectionMetadata        InferenceContextSection = "metadata"
	InferenceContextSectionTokens          InferenceContextSection = "tokens"
	InferenceContextSectionRequestHeaders  InferenceContextSection = "request_headers"
	InferenceContextSectionResponseHeaders InferenceContextSection = "response_headers"
)

func SupportedInferenceContextSections() []InferenceContextSection {
	return []InferenceContextSection{
		InferenceContextSectionMetadata,
		InferenceContextSectionTokens,
		InferenceContextSectionRequestHeaders,
		InferenceContextSectionResponseHeaders,
	}
}

type InferenceContextBodyName string

const (
	InferenceContextBodyRequestBody  InferenceContextBodyName = "request_body"
	InferenceContextBodyResponseBody InferenceContextBodyName = "response_body"
)

func SupportedInferenceContextBodies() []InferenceContextBodyName {
	return []InferenceContextBodyName{
		InferenceContextBodyRequestBody,
		InferenceContextBodyResponseBody,
	}
}

type InferenceContextBodyRequest struct {
	Name   InferenceContextBodyName
	Offset int
	Limit  int
}

func (r InferenceContextBodyRequest) Normalized() InferenceContextBodyRequest {
	if r.Offset < 0 {
		r.Offset = 0
	}
	if r.Limit <= 0 {
		r.Limit = defaultInferenceContextBodyLimit
	}
	return r
}

type InferenceContextAvailability struct {
	Metadata        bool `json:"metadata"`
	Tokens          bool `json:"tokens"`
	RequestHeaders  bool `json:"requestHeaders"`
	ResponseHeaders bool `json:"responseHeaders"`
	RequestBody     bool `json:"requestBody"`
	ResponseBody    bool `json:"responseBody"`
}

type InferenceContextMetadata struct {
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

type InferenceContextBodyChunk struct {
	Name       InferenceContextBodyName `json:"name"`
	Offset     int                      `json:"offset"`
	Limit      int                      `json:"limit"`
	NextOffset int                      `json:"nextOffset"`
	HasMore    bool                     `json:"hasMore"`
	TotalBytes int                      `json:"totalBytes"`
	Content    string                   `json:"content"`
	Truncated  bool                     `json:"truncated"`
}

type GetInferenceContextQuery struct {
	ID       string
	Sections []InferenceContextSection
	Body     *InferenceContextBodyRequest
}

type GetInferenceContextResult struct {
	AvailableSections InferenceContextAvailability `json:"availableSections"`
	Metadata          *InferenceContextMetadata    `json:"metadata,omitempty"`
	Tokens            *inference.TokenStats        `json:"tokens,omitempty"`
	RequestHeaders    []inference.Header           `json:"requestHeaders,omitempty"`
	ResponseHeaders   []inference.Header           `json:"responseHeaders,omitempty"`
	BodyChunk         *InferenceContextBodyChunk   `json:"bodyChunk,omitempty"`
}
