package ollama

import (
	"context"
	"sync"
	"time"
)

type Poller struct {
	mu          sync.Mutex
	client      *Client
	clock       Clock
	latest      PollSnapshot
	showDigests map[string]string
	showCache   map[string]ShowSnapshot
}

func NewPoller(client *Client, clock Clock) *Poller {
	if clock == nil {
		clock = time.Now
	}

	return &Poller{
		client:      client,
		clock:       clock,
		showDigests: map[string]string{},
		showCache:   map[string]ShowSnapshot{},
	}
}

func (poller *Poller) Poll(ctx context.Context, request PollRequest) PollSnapshot {
	poller.mu.Lock()
	defer poller.mu.Unlock()

	result := PollSnapshot{
		Shows: map[string]ShowSnapshot{},
	}

	version, versionErr := poller.client.Version(ctx)
	if versionErr != nil {
		result.Version = staleVersion(poller.latest.Version, poller.clock(), versionErr.Error())
	} else {
		result.Version = version
		poller.latest.Version = version
	}

	running, runningErr := poller.client.Running(ctx)
	if runningErr != nil {
		result.Running = staleRunning(poller.latest.Running, result.Version.Meta.ObservedAt, runningErr.Error())
	} else {
		result.Running = running
		poller.latest.Running = running
	}

	catalog, catalogErr := poller.client.Catalog(ctx)
	if catalogErr != nil {
		result.Catalog = staleCatalog(poller.latest.Catalog, result.Version.Meta.ObservedAt, catalogErr.Error())
	} else {
		result.Catalog = catalog
		poller.latest.Catalog = catalog
	}

	for _, model := range request.ShowModels {
		digest := catalogDigest(result.Catalog.Models, model)
		cached, ok := poller.showCache[model]
		if ok && digest != "" && poller.showDigests[model] == digest {
			result.Shows[model] = cachedShow(cached, poller.clock())
			continue
		}

		show, err := poller.client.Show(ctx, model)
		if err != nil {
			result.Shows[model] = staleShow(cached, result.Version.Meta.ObservedAt, model, err.Error())
			continue
		}

		result.Shows[model] = show
		poller.showCache[model] = show
		if digest != "" {
			poller.showDigests[model] = digest
		}
		poller.latest.Shows = ensureShowsMap(poller.latest.Shows)
		poller.latest.Shows[model] = show
	}

	result.Meta = pollMeta(result, versionErr, runningErr, catalogErr)
	if result.Meta.Status == StatusConfirmed {
		poller.latest.Meta = result.Meta
	}

	return result
}

func ensureShowsMap(values map[string]ShowSnapshot) map[string]ShowSnapshot {
	if values == nil {
		return map[string]ShowSnapshot{}
	}
	return values
}

func pollMeta(result PollSnapshot, versionErr error, runningErr error, catalogErr error) SnapshotMeta {
	meta := result.Version.Meta
	if meta.Endpoint == "" {
		meta = result.Running.Meta
	}
	if meta.Endpoint == "" {
		meta = result.Catalog.Meta
	}

	if versionErr == nil && runningErr == nil && catalogErr == nil {
		meta.Status = StatusConfirmed
		meta.Reachable = true
		meta.Error = ""
		return meta
	}

	meta.Status = StatusUnreachable
	meta.Reachable = false
	if versionErr != nil {
		meta.Error = versionErr.Error()
	} else if runningErr != nil {
		meta.Error = runningErr.Error()
	} else if catalogErr != nil {
		meta.Error = catalogErr.Error()
	}

	return meta
}

func catalogDigest(models []CatalogModel, name string) string {
	for _, model := range models {
		if model.Name == name || model.Model == name {
			return model.Digest
		}
	}
	return ""
}

func cachedShow(snapshot ShowSnapshot, observedAt time.Time) ShowSnapshot {
	snapshot.Meta.ObservedAt = observedAt
	snapshot.Meta.Cached = true
	snapshot.Meta.Status = StatusConfirmed
	snapshot.Meta.Reachable = true
	snapshot.Meta.Error = ""
	return snapshot
}

func staleVersion(previous VersionSnapshot, observedAt time.Time, message string) VersionSnapshot {
	previous.Meta = staleMeta(previous.Meta, observedAt, message)
	return previous
}

func staleRunning(previous RunningModelsSnapshot, observedAt time.Time, message string) RunningModelsSnapshot {
	previous.Meta = staleMeta(previous.Meta, observedAt, message)
	return previous
}

func staleCatalog(previous CatalogSnapshot, observedAt time.Time, message string) CatalogSnapshot {
	previous.Meta = staleMeta(previous.Meta, observedAt, message)
	return previous
}

func staleShow(previous ShowSnapshot, observedAt time.Time, model string, message string) ShowSnapshot {
	previous.Model = model
	previous.Meta = staleMeta(previous.Meta, observedAt, message)
	return previous
}

func staleMeta(meta SnapshotMeta, observedAt time.Time, message string) SnapshotMeta {
	meta.Source = SourceHTTPAPI
	meta.ObservedAt = observedAt
	meta.Status = StatusUnreachable
	meta.Reachable = false
	meta.Cached = false
	meta.Error = message
	return meta
}
