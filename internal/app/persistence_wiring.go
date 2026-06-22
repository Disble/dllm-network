package app

import (
	"context"

	"dllm-network/internal/events"
	"dllm-network/internal/persistence"
	"dllm-network/internal/store/sqlite"
)

// closer is satisfied by *sqlite.Store and any test double that needs to
// release resources on shutdown.
type closer interface {
	Close() error
}

// persistenceLifecycle owns the durable-write side of the async write path
// (design D7): the sqlite.Store writer connection plus the Subscriber that
// drains the shared bus topic into batched INSERTs. It is started from
// App.Startup and stopped from App.Quit, mirroring inferencePipeline's
// run/stop shape.
type persistenceLifecycle struct {
	subscriber *persistence.Subscriber
	store      closer
	unsubscribe func()

	cancel context.CancelFunc
	done   chan struct{}
}

// newPersistenceLifecycle wires a Subscriber around writer and returns the
// lifecycle handle. writer is also wrapped as the closer to release on
// Stop, when it implements closer (the real sqlite.Store always does; test
// doubles may opt in).
func newPersistenceLifecycle(writer persistence.Writer) *persistenceLifecycle {
	sub := persistence.NewSubscriber(writer)

	var store closer
	if c, ok := writer.(closer); ok {
		store = c
	}

	return &persistenceLifecycle{
		subscriber: sub,
		store:      store,
		done:       make(chan struct{}),
	}
}

// start subscribes the bus and launches the drain-loop goroutine.
func (pl *persistenceLifecycle) start(ctx context.Context, bus *events.Bus) {
	pl.unsubscribe = pl.subscriber.Subscribe(bus)

	runCtx, cancel := context.WithCancel(ctx)
	pl.cancel = cancel

	go func() {
		defer close(pl.done)
		pl.subscriber.Run(runCtx)
	}()
}

// stop unsubscribes from the bus, stops the drain loop (final flush), waits
// for it to exit, and closes the underlying store.
func (pl *persistenceLifecycle) stop() {
	if pl.unsubscribe != nil {
		pl.unsubscribe()
	}
	pl.subscriber.Stop()
	if pl.cancel != nil {
		pl.cancel()
	}
	<-pl.done
	if pl.store != nil {
		_ = pl.store.Close()
	}
}

// defaultDBPath resolves the production SQLite database path under the
// user's local (non-roaming) data directory, creating the parent directory
// if needed. It delegates to sqlite.DefaultPath(), the SHARED resolver both
// this GUI writer and the stdio sidecar (cmd/dllm-network-mcp) use, so
// the two processes can never resolve different files. defaultDBPath exists
// as a thin wrapper (rather than every caller importing sqlite directly)
// purely to keep this package's existing call sites unchanged.
func defaultDBPath() (string, error) {
	return sqlite.DefaultPath()
}

// openDefaultStore opens the production writer Store at the resolved
// default path. Returns an error the caller can choose to tolerate
// (degraded mode: app runs without durable persistence) rather than crash
// the whole app on a storage failure.
func openDefaultStore() (*sqlite.Store, error) {
	path, err := defaultDBPath()
	if err != nil {
		return nil, err
	}
	return sqlite.Open(path)
}
