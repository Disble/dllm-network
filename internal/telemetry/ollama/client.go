package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	clock      Clock
}

func NewClient(baseURL string, httpClient *http.Client, clock Clock) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if clock == nil {
		clock = time.Now
	}

	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
		clock:      clock,
	}
}

func (client *Client) Version(ctx context.Context) (VersionSnapshot, error) {
	var payload struct {
		Version string `json:"version"`
	}

	endpoint, err := client.get(ctx, "/api/version", &payload)
	if err != nil {
		return VersionSnapshot{}, err
	}

	observedAt := client.clock()
	return VersionSnapshot{
		Meta:    confirmedMeta(endpoint, observedAt),
		Version: payload.Version,
	}, nil
}

func (client *Client) Running(ctx context.Context) (RunningModelsSnapshot, error) {
	var payload struct {
		Models []struct {
			Name          string       `json:"name"`
			Model         string       `json:"model"`
			Size          int64        `json:"size"`
			Digest        string       `json:"digest"`
			Details       ModelDetails `json:"details"`
			ExpiresAt     string       `json:"expires_at"`
			SizeVRAM      int64        `json:"size_vram"`
			ContextLength int          `json:"context_length"`
		} `json:"models"`
	}

	endpoint, err := client.get(ctx, "/api/ps", &payload)
	if err != nil {
		return RunningModelsSnapshot{}, err
	}

	models := make([]RunningModel, 0, len(payload.Models))
	for _, model := range payload.Models {
		models = append(models, RunningModel{
			Name:          model.Name,
			Model:         model.Model,
			Size:          model.Size,
			Digest:        model.Digest,
			Details:       model.Details,
			ExpiresAt:     parseTimestamp(model.ExpiresAt),
			SizeVRAM:      model.SizeVRAM,
			ContextLength: model.ContextLength,
		})
	}

	observedAt := client.clock()
	return RunningModelsSnapshot{
		Meta:   confirmedMeta(endpoint, observedAt),
		Models: models,
	}, nil
}

func (client *Client) Catalog(ctx context.Context) (CatalogSnapshot, error) {
	var payload struct {
		Models []struct {
			Name        string       `json:"name"`
			Model       string       `json:"model"`
			RemoteModel string       `json:"remote_model"`
			RemoteHost  string       `json:"remote_host"`
			ModifiedAt  string       `json:"modified_at"`
			Size        int64        `json:"size"`
			Digest      string       `json:"digest"`
			Details     ModelDetails `json:"details"`
		} `json:"models"`
	}

	endpoint, err := client.get(ctx, "/api/tags", &payload)
	if err != nil {
		return CatalogSnapshot{}, err
	}

	models := make([]CatalogModel, 0, len(payload.Models))
	for _, model := range payload.Models {
		models = append(models, CatalogModel{
			Name:        model.Name,
			Model:       model.Model,
			RemoteModel: model.RemoteModel,
			RemoteHost:  model.RemoteHost,
			ModifiedAt:  parseTimestamp(model.ModifiedAt),
			Size:        model.Size,
			Digest:      model.Digest,
			Details:     model.Details,
		})
	}

	observedAt := client.clock()
	return CatalogSnapshot{
		Meta:   confirmedMeta(endpoint, observedAt),
		Models: models,
	}, nil
}

func (client *Client) Show(ctx context.Context, model string) (ShowSnapshot, error) {
	payload := struct {
		Model string `json:"model"`
	}{Model: model}

	var response struct {
		Parameters   string         `json:"parameters"`
		License      string         `json:"license"`
		Template     string         `json:"template"`
		Capabilities []string       `json:"capabilities"`
		ModifiedAt   string         `json:"modified_at"`
		Details      ModelDetails   `json:"details"`
		ModelInfo    map[string]any `json:"model_info"`
	}

	endpoint, err := client.post(ctx, "/api/show", payload, &response)
	if err != nil {
		return ShowSnapshot{}, err
	}

	observedAt := client.clock()
	return ShowSnapshot{
		Meta:         confirmedMeta(endpoint, observedAt),
		Model:        model,
		Parameters:   response.Parameters,
		License:      response.License,
		Template:     response.Template,
		Capabilities: response.Capabilities,
		ModifiedAt:   parseTimestamp(response.ModifiedAt),
		Details:      response.Details,
		ModelInfo:    cloneModelInfo(response.ModelInfo),
	}, nil
}

func (client *Client) get(ctx context.Context, path string, target any) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.endpoint(path), nil)
	if err != nil {
		return "", err
	}

	return client.do(request, target)
}

func (client *Client) post(ctx context.Context, path string, payload any, target any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint(path), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	return client.do(request, target)
}

func (client *Client) do(request *http.Request, target any) (string, error) {
	response, err := client.httpClient.Do(request)
	if err != nil {
		return request.URL.String(), fmt.Errorf("request %s failed: %w", request.URL.String(), err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return request.URL.String(), fmt.Errorf("request %s returned %s: %s", request.URL.String(), response.Status, strings.TrimSpace(string(body)))
	}

	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(target); err != nil {
		return request.URL.String(), fmt.Errorf("decode %s failed: %w", request.URL.String(), err)
	}

	return request.URL.String(), nil
}

func (client *Client) endpoint(path string) string {
	if client.baseURL == "" {
		return path
	}
	return client.baseURL + path
}

func confirmedMeta(endpoint string, observedAt time.Time) SnapshotMeta {
	return SnapshotMeta{
		Source:          SourceHTTPAPI,
		Endpoint:        endpoint,
		ObservedAt:      observedAt,
		LastConfirmedAt: observedAt,
		Status:          StatusConfirmed,
		Reachable:       true,
	}
}

func parseTimestamp(value string) time.Time {
	if value == "" {
		return time.Time{}
	}

	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}

	return parsed
}

func cloneModelInfo(source map[string]any) map[string]any {
	if len(source) == 0 {
		return map[string]any{}
	}

	target := make(map[string]any, len(source))
	for key, value := range source {
		target[key] = value
	}

	return target
}
