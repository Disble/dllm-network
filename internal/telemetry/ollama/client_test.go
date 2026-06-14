package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientVersionParsesConfirmedSnapshot(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		baseURL    func(*httptest.Server) string
		payload    string
		observedAt time.Time
		want       string
	}{
		{
			name:       "parses semantic version response",
			baseURL:    func(server *httptest.Server) string { return server.URL },
			payload:    `{"version":"0.12.6"}`,
			observedAt: time.Date(2026, time.June, 14, 20, 0, 0, 0, time.UTC),
			want:       "0.12.6",
		},
		{
			name:       "normalizes trailing slash base url",
			baseURL:    func(server *httptest.Server) string { return server.URL + "/" },
			payload:    `{"version":"1.0.0-rc1"}`,
			observedAt: time.Date(2026, time.June, 14, 20, 1, 0, 0, time.UTC),
			want:       "1.0.0-rc1",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet {
					t.Fatalf("expected GET request, got %s", request.Method)
				}
				if request.URL.Path != "/api/version" {
					t.Fatalf("expected /api/version path, got %s", request.URL.Path)
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(tt.payload))
			}))
			defer server.Close()

			client := NewClient(tt.baseURL(server), server.Client(), fixedClock(tt.observedAt))

			snapshot, err := client.Version(context.Background())
			if err != nil {
				t.Fatalf("version: %v", err)
			}

			if snapshot.Version != tt.want {
				t.Fatalf("expected version %q, got %q", tt.want, snapshot.Version)
			}
			assertConfirmedMeta(t, snapshot.Meta, tt.observedAt, server.URL+"/api/version")
		})
	}
}

func TestClientRunningParsesLoadedModelSnapshots(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		payload    string
		observedAt time.Time
		assert     func(*testing.T, RunningModelsSnapshot)
	}{
		{
			name:       "parses full running model payload",
			payload:    `{"models":[{"name":"gemma4","model":"gemma4","size":6591830464,"digest":"abc123","details":{"parent_model":"","format":"gguf","family":"gemma4","families":["gemma4"],"parameter_size":"8.0B","quantization_level":"Q4_K_M"},"expires_at":"2025-10-17T16:47:07.93355-07:00","size_vram":5333539264,"context_length":4096}]}`,
			observedAt: time.Date(2026, time.June, 14, 20, 2, 0, 0, time.UTC),
			assert: func(t *testing.T, snapshot RunningModelsSnapshot) {
				t.Helper()
				if len(snapshot.Models) != 1 {
					t.Fatalf("expected one model, got %d", len(snapshot.Models))
				}
				model := snapshot.Models[0]
				if model.SizeVRAM != 5333539264 || model.ContextLength != 4096 {
					t.Fatalf("expected VRAM/context parsing, got %+v", model)
				}
				if model.Digest != "abc123" {
					t.Fatalf("expected digest abc123, got %q", model.Digest)
				}
				if model.Details.Family != "gemma4" || model.Details.QuantizationLevel != "Q4_K_M" {
					t.Fatalf("expected detail fields, got %+v", model.Details)
				}
				wantExpiry := time.Date(2025, time.October, 17, 16, 47, 7, 933550000, time.FixedZone("-0700", -7*60*60))
				if !model.ExpiresAt.Equal(wantExpiry) {
					t.Fatalf("expected expires_at %s, got %s", wantExpiry, model.ExpiresAt)
				}
			},
		},
		{
			name:       "handles sparse running model payload",
			payload:    `{"models":[{"name":"tiny","model":"tiny","size":42}]}`,
			observedAt: time.Date(2026, time.June, 14, 20, 3, 0, 0, time.UTC),
			assert: func(t *testing.T, snapshot RunningModelsSnapshot) {
				t.Helper()
				if len(snapshot.Models) != 1 {
					t.Fatalf("expected one model, got %d", len(snapshot.Models))
				}
				model := snapshot.Models[0]
				if model.Size != 42 || model.SizeVRAM != 0 || model.ContextLength != 0 {
					t.Fatalf("expected sparse numeric fields to parse cleanly, got %+v", model)
				}
				if !model.ExpiresAt.IsZero() {
					t.Fatalf("expected zero expires_at for sparse payload, got %s", model.ExpiresAt)
				}
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/api/ps" {
					t.Fatalf("expected /api/ps path, got %s", request.URL.Path)
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(tt.payload))
			}))
			defer server.Close()

			client := NewClient(server.URL, server.Client(), fixedClock(tt.observedAt))

			snapshot, err := client.Running(context.Background())
			if err != nil {
				t.Fatalf("running: %v", err)
			}

			assertConfirmedMeta(t, snapshot.Meta, tt.observedAt, server.URL+"/api/ps")
			tt.assert(t, snapshot)
		})
	}
}

func TestClientCatalogParsesLocalModelSnapshots(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		payload    string
		observedAt time.Time
		assert     func(*testing.T, CatalogSnapshot)
	}{
		{
			name:       "parses detailed local model catalog",
			payload:    `{"models":[{"name":"gemma4","model":"gemma4","modified_at":"2025-10-03T23:34:03.409490317-07:00","size":9608350245,"digest":"digest-1","details":{"format":"gguf","family":"gemma4","families":["gemma4"],"parameter_size":"8.0B","quantization_level":"Q4_K_M"}}]}`,
			observedAt: time.Date(2026, time.June, 14, 20, 4, 0, 0, time.UTC),
			assert: func(t *testing.T, snapshot CatalogSnapshot) {
				t.Helper()
				if len(snapshot.Models) != 1 {
					t.Fatalf("expected one model, got %d", len(snapshot.Models))
				}
				model := snapshot.Models[0]
				if model.Size != 9608350245 || model.Digest != "digest-1" {
					t.Fatalf("expected size and digest to parse, got %+v", model)
				}
				if model.Details.Family != "gemma4" || model.Details.QuantizationLevel != "Q4_K_M" {
					t.Fatalf("expected model family and quantization, got %+v", model.Details)
				}
				wantModified := time.Date(2025, time.October, 3, 23, 34, 3, 409490317, time.FixedZone("-0700", -7*60*60))
				if !model.ModifiedAt.Equal(wantModified) {
					t.Fatalf("expected modified_at %s, got %s", wantModified, model.ModifiedAt)
				}
			},
		},
		{
			name:       "handles sparse catalog payload",
			payload:    `{"models":[{"name":"tiny","model":"tiny","size":21}]}`,
			observedAt: time.Date(2026, time.June, 14, 20, 5, 0, 0, time.UTC),
			assert: func(t *testing.T, snapshot CatalogSnapshot) {
				t.Helper()
				if len(snapshot.Models) != 1 {
					t.Fatalf("expected one model, got %d", len(snapshot.Models))
				}
				model := snapshot.Models[0]
				if model.Size != 21 {
					t.Fatalf("expected size 21, got %+v", model)
				}
				if !model.ModifiedAt.IsZero() {
					t.Fatalf("expected zero modified_at for sparse payload, got %s", model.ModifiedAt)
				}
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/api/tags" {
					t.Fatalf("expected /api/tags path, got %s", request.URL.Path)
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(tt.payload))
			}))
			defer server.Close()

			client := NewClient(server.URL, server.Client(), fixedClock(tt.observedAt))

			snapshot, err := client.Catalog(context.Background())
			if err != nil {
				t.Fatalf("catalog: %v", err)
			}

			assertConfirmedMeta(t, snapshot.Meta, tt.observedAt, server.URL+"/api/tags")
			tt.assert(t, snapshot)
		})
	}
}

func TestClientShowParsesModelMetadataWhenRequested(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		payload    string
		observedAt time.Time
		assert     func(*testing.T, ShowSnapshot)
	}{
		{
			name:       "parses detailed show payload",
			payload:    `{"parameters":"temperature 0.7\nnum_ctx 2048","license":"gemma terms","template":"{{ .Prompt }}","capabilities":["completion","vision"],"modified_at":"2025-08-14T15:49:43.634137516-07:00","details":{"format":"gguf","family":"gemma4","quantization_level":"Q4_K_M"},"model_info":{"general.architecture":"gemma4","gemma4.context_length":131072}}`,
			observedAt: time.Date(2026, time.June, 14, 20, 6, 0, 0, time.UTC),
			assert: func(t *testing.T, snapshot ShowSnapshot) {
				t.Helper()
				if snapshot.Parameters == "" || snapshot.Template == "" {
					t.Fatalf("expected parameters and template, got %+v", snapshot)
				}
				if len(snapshot.Capabilities) != 2 || snapshot.Capabilities[1] != "vision" {
					t.Fatalf("expected capabilities to parse, got %v", snapshot.Capabilities)
				}
				if snapshot.Details.Family != "gemma4" || snapshot.Details.QuantizationLevel != "Q4_K_M" {
					t.Fatalf("expected family and quantization, got %+v", snapshot.Details)
				}
				if got := snapshot.ModelInfo["general.architecture"]; got != "gemma4" {
					t.Fatalf("expected architecture metadata, got %v", got)
				}
			},
		},
		{
			name:       "handles sparse show payload",
			payload:    `{"capabilities":["completion"]}`,
			observedAt: time.Date(2026, time.June, 14, 20, 7, 0, 0, time.UTC),
			assert: func(t *testing.T, snapshot ShowSnapshot) {
				t.Helper()
				if len(snapshot.Capabilities) != 1 || snapshot.Capabilities[0] != "completion" {
					t.Fatalf("expected single capability, got %v", snapshot.Capabilities)
				}
				if snapshot.Parameters != "" || snapshot.License != "" || len(snapshot.ModelInfo) != 0 {
					t.Fatalf("expected sparse fields to remain zero values, got %+v", snapshot)
				}
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/api/show" {
					t.Fatalf("expected /api/show path, got %s", request.URL.Path)
				}
				if request.Method != http.MethodPost {
					t.Fatalf("expected POST request, got %s", request.Method)
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(tt.payload))
			}))
			defer server.Close()

			client := NewClient(server.URL, server.Client(), fixedClock(tt.observedAt))

			snapshot, err := client.Show(context.Background(), "gemma4")
			if err != nil {
				t.Fatalf("show: %v", err)
			}

			if snapshot.Model != "gemma4" {
				t.Fatalf("expected model name gemma4, got %q", snapshot.Model)
			}
			assertConfirmedMeta(t, snapshot.Meta, tt.observedAt, server.URL+"/api/show")
			tt.assert(t, snapshot)
		})
	}
}

func assertConfirmedMeta(t *testing.T, meta SnapshotMeta, observedAt time.Time, wantEndpoint string) {
	t.Helper()

	if meta.Source != SourceHTTPAPI {
		t.Fatalf("expected source %q, got %q", SourceHTTPAPI, meta.Source)
	}
	if meta.Endpoint != wantEndpoint {
		t.Fatalf("expected endpoint %q, got %q", wantEndpoint, meta.Endpoint)
	}
	if meta.Status != StatusConfirmed {
		t.Fatalf("expected confirmed status, got %q", meta.Status)
	}
	if !meta.Reachable {
		t.Fatal("expected confirmed snapshot to be reachable")
	}
	if meta.Cached {
		t.Fatal("expected direct client snapshot to be uncached")
	}
	if !meta.ObservedAt.Equal(observedAt) {
		t.Fatalf("expected observed_at %s, got %s", observedAt, meta.ObservedAt)
	}
	if !meta.LastConfirmedAt.Equal(observedAt) {
		t.Fatalf("expected last_confirmed_at %s, got %s", observedAt, meta.LastConfirmedAt)
	}
	if meta.Error != "" {
		t.Fatalf("expected empty error, got %q", meta.Error)
	}
}

func fixedClock(now time.Time) Clock {
	return func() time.Time { return now }
}

func containsText(value string, fragment string) bool {
	return strings.Contains(value, fragment)
}
