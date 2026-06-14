package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestPollerCachesShowResponsesWhenCatalogDigestIsUnchanged(t *testing.T) {
	t.Parallel()

	var showCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		switch request.URL.Path {
		case "/api/version":
			_, _ = writer.Write([]byte(`{"version":"0.12.6"}`))
		case "/api/ps":
			_, _ = writer.Write([]byte(`{"models":[{"name":"gemma4","model":"gemma4","size":100,"digest":"run-digest"}]}`))
		case "/api/tags":
			_, _ = writer.Write([]byte(`{"models":[{"name":"gemma4","model":"gemma4","size":100,"digest":"catalog-digest","details":{"family":"gemma4"}}]}`))
		case "/api/show":
			showCalls.Add(1)
			_, _ = writer.Write([]byte(`{"capabilities":["completion","vision"],"details":{"family":"gemma4"}}`))
		default:
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
	}))
	defer server.Close()

	clock := sequenceClock(
		time.Date(2026, time.June, 14, 20, 10, 0, 0, time.UTC),
		time.Date(2026, time.June, 14, 20, 11, 0, 0, time.UTC),
		time.Date(2026, time.June, 14, 20, 12, 0, 0, time.UTC),
		time.Date(2026, time.June, 14, 20, 13, 0, 0, time.UTC),
		time.Date(2026, time.June, 14, 20, 14, 0, 0, time.UTC),
		time.Date(2026, time.June, 14, 20, 15, 0, 0, time.UTC),
		time.Date(2026, time.June, 14, 20, 16, 0, 0, time.UTC),
		time.Date(2026, time.June, 14, 20, 17, 0, 0, time.UTC),
	)
	client := NewClient(server.URL, server.Client(), clock)
	poller := NewPoller(client, clock)

	first := poller.Poll(context.Background(), PollRequest{ShowModels: []string{"gemma4"}})
	second := poller.Poll(context.Background(), PollRequest{ShowModels: []string{"gemma4"}})

	if showCalls.Load() != 1 {
		t.Fatalf("expected one /api/show request across two polls, got %d", showCalls.Load())
	}

	showSnapshot, ok := second.Shows["gemma4"]
	if !ok {
		t.Fatal("expected show snapshot for gemma4")
	}
	if !showSnapshot.Meta.Cached {
		t.Fatal("expected second show snapshot to be served from cache")
	}
	if showSnapshot.Meta.Status != StatusConfirmed || !showSnapshot.Meta.Reachable {
		t.Fatalf("expected cached snapshot to remain confirmed and reachable, got %+v", showSnapshot.Meta)
	}
	if !showSnapshot.Meta.LastConfirmedAt.Equal(first.Shows["gemma4"].Meta.LastConfirmedAt) {
		t.Fatalf("expected cache hit to preserve original confirmation time, got %s vs %s", showSnapshot.Meta.LastConfirmedAt, first.Shows["gemma4"].Meta.LastConfirmedAt)
	}
	if !showSnapshot.Meta.ObservedAt.After(first.Shows["gemma4"].Meta.ObservedAt) {
		t.Fatalf("expected cache hit to refresh observed_at, got %s after %s", showSnapshot.Meta.ObservedAt, first.Shows["gemma4"].Meta.ObservedAt)
	}
}

func TestPollerReturnsUnreachableSnapshotAndRetainsConfirmedTimestamps(t *testing.T) {
	t.Parallel()

	clock := sequenceClock(
		time.Date(2026, time.June, 14, 20, 20, 0, 0, time.UTC),
		time.Date(2026, time.June, 14, 20, 21, 0, 0, time.UTC),
		time.Date(2026, time.June, 14, 20, 22, 0, 0, time.UTC),
		time.Date(2026, time.June, 14, 20, 23, 0, 0, time.UTC),
		time.Date(2026, time.June, 14, 20, 24, 0, 0, time.UTC),
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		switch request.URL.Path {
		case "/api/version":
			_, _ = writer.Write([]byte(`{"version":"0.12.6"}`))
		case "/api/ps":
			_, _ = writer.Write([]byte(`{"models":[{"name":"gemma4","model":"gemma4","size":100,"size_vram":50,"context_length":4096}]}`))
		case "/api/tags":
			_, _ = writer.Write([]byte(`{"models":[{"name":"gemma4","model":"gemma4","size":100,"digest":"catalog-digest"}]}`))
		default:
			t.Fatalf("unexpected path %s", request.URL.Path)
		}
	}))

	client := NewClient(server.URL, server.Client(), clock)
	poller := NewPoller(client, clock)

	confirmed := poller.Poll(context.Background(), PollRequest{})
	server.Close()

	unreachable := poller.Poll(context.Background(), PollRequest{})

	if unreachable.Meta.Status != StatusUnreachable || unreachable.Meta.Reachable {
		t.Fatalf("expected unreachable poll status, got %+v", unreachable.Meta)
	}
	if unreachable.Meta.Error == "" {
		t.Fatal("expected unreachable snapshot to capture an error message")
	}
	if unreachable.Version.Version != confirmed.Version.Version {
		t.Fatalf("expected stale version %q to be retained, got %q", confirmed.Version.Version, unreachable.Version.Version)
	}
	if !unreachable.Version.Meta.LastConfirmedAt.Equal(confirmed.Version.Meta.LastConfirmedAt) {
		t.Fatalf("expected last confirmed timestamp to be retained, got %s vs %s", unreachable.Version.Meta.LastConfirmedAt, confirmed.Version.Meta.LastConfirmedAt)
	}
	if !unreachable.Version.Meta.ObservedAt.After(confirmed.Version.Meta.ObservedAt) {
		t.Fatalf("expected unreachable observed_at to advance, got %s after %s", unreachable.Version.Meta.ObservedAt, confirmed.Version.Meta.ObservedAt)
	}
	if unreachable.Version.Meta.Source != SourceHTTPAPI {
		t.Fatalf("expected retained source %q, got %q", SourceHTTPAPI, unreachable.Version.Meta.Source)
	}
	if unreachable.Running.Models[0].ContextLength != 4096 {
		t.Fatalf("expected stale running model data to be retained, got %+v", unreachable.Running.Models[0])
	}
	if unreachable.Catalog.Models[0].Digest != "catalog-digest" {
		t.Fatalf("expected stale catalog digest to be retained, got %+v", unreachable.Catalog.Models[0])
	}
	if unreachable.Version.Meta.Status != StatusUnreachable || unreachable.Running.Meta.Status != StatusUnreachable || unreachable.Catalog.Meta.Status != StatusUnreachable {
		t.Fatalf("expected all retained snapshots to be marked unreachable, got version=%+v running=%+v catalog=%+v", unreachable.Version.Meta, unreachable.Running.Meta, unreachable.Catalog.Meta)
	}
	if !containsText(unreachable.Version.Meta.Error, "/api/version") {
		t.Fatalf("expected unreachable error to reference endpoint, got %q", unreachable.Version.Meta.Error)
	}
}

func sequenceClock(values ...time.Time) Clock {
	var index atomic.Int32

	return func() time.Time {
		current := int(index.Add(1)) - 1
		if current >= len(values) {
			return values[len(values)-1]
		}
		return values[current]
	}
}
