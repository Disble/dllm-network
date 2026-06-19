package sqlite

import (
	"encoding/json"
	"fmt"

	"ollama-telemetry/internal/telemetry/inference"
)

// detail is the JSON-encoded payload stored in the inferences.detail column.
// It carries everything that is heavy or never filtered/sorted on: request
// and response bodies (with their truncation flags) and the captured
// headers. The full TokenStats is intentionally duplicated here (alongside
// the flattened scalar columns in schema.go) so unmarshalDetail can rebuild
// *inference.TokenStats with a single nil check, without re-deriving it from
// individually-NULLable scalar columns.
type detail struct {
	Tokens                *inference.TokenStats `json:"tokens"`
	Generation            *inference.Generation `json:"generation"`
	RequestBody           string                `json:"requestBody"`
	RequestBodyTruncated  bool                  `json:"requestBodyTruncated"`
	ResponseBody          string                `json:"responseBody"`
	ResponseBodyTruncated bool                  `json:"responseBodyTruncated"`
	RequestHeaders        []inference.Header    `json:"requestHeaders"`
	ResponseHeaders       []inference.Header    `json:"responseHeaders"`
}

// marshalDetail isolates the inference.Inference <-> JSON mapping in one
// place so the domain type (internal/telemetry/inference) never needs to
// import database/sql or know about this storage layout.
func marshalDetail(inf inference.Inference) (string, error) {
	d := detail{
		Tokens:                inf.Tokens,
		Generation:            inf.Generation,
		RequestBody:           inf.RequestBody,
		RequestBodyTruncated:  inf.RequestBodyTruncated,
		ResponseBody:          inf.ResponseBody,
		ResponseBodyTruncated: inf.ResponseBodyTruncated,
		RequestHeaders:        inf.RequestHeaders,
		ResponseHeaders:       inf.ResponseHeaders,
	}

	raw, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("sqlite: marshal detail: %w", err)
	}
	return string(raw), nil
}

// unmarshalDetail decodes the detail JSON blob and applies it onto inf,
// preserving the nil-vs-zero contracts (Tokens, RequestHeaders,
// ResponseHeaders) that the domain type requires.
func unmarshalDetail(raw string, inf *inference.Inference) error {
	var d detail
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return fmt.Errorf("sqlite: unmarshal detail: %w", err)
	}

	inf.Tokens = d.Tokens
	inf.Generation = d.Generation
	inf.RequestBody = d.RequestBody
	inf.RequestBodyTruncated = d.RequestBodyTruncated
	inf.ResponseBody = d.ResponseBody
	inf.ResponseBodyTruncated = d.ResponseBodyTruncated
	inf.RequestHeaders = d.RequestHeaders
	inf.ResponseHeaders = d.ResponseHeaders

	return nil
}
