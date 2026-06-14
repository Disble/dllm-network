package system

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"strings"
)

type ConnectionCollector struct {
	provider ConnectionProvider
	clock    Clock
}

func NewConnectionCollector(provider ConnectionProvider, clock Clock) *ConnectionCollector {
	if provider == nil {
		provider = defaultConnectionProvider{}
	}

	return &ConnectionCollector{
		provider: provider,
		clock:    resolveClock(clock),
	}
}

func (collector *ConnectionCollector) Collect(ctx context.Context, ownerPID int32) ConnectionsSnapshot {
	observedAt := collector.clock()
	connections, err := collector.provider.Connections(ctx)
	if err != nil {
		return ConnectionsSnapshot{
			Meta: metaFailure(SourceConnections, DefaultConnectionProvider, observedAt, err),
			PID:  ownerPID,
		}
	}

	return ConnectionsSnapshot{
		Meta:        metaSuccess(SourceConnections, DefaultConnectionProvider, observedAt),
		PID:         ownerPID,
		Connections: ownedLoopbackConnections(connections, ownerPID),
	}
}

func ownedLoopbackConnections(connections []ConnectionSample, ownerPID int32) []ConnectionSample {
	filtered := make([]ConnectionSample, 0, len(connections))
	for _, connection := range connections {
		if connection.PID != ownerPID {
			continue
		}
		if !isLoopbackAddress(connection.LocalAddress) {
			continue
		}
		if !isLoopbackAddress(connection.RemoteAddress) {
			continue
		}

		filtered = append(filtered, connection)
	}

	return filtered
}

func isLoopbackAddress(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}

	parsed := net.ParseIP(trimmed)
	if parsed == nil {
		return false
	}

	return parsed.IsLoopback()
}

type defaultConnectionProvider struct{}

func (defaultConnectionProvider) Connections(context.Context) ([]ConnectionSample, error) {
	return nil, fmt.Errorf("%s platform connection ownership is not wired yet: %w", runtime.GOOS, ErrUnsupported)
}
