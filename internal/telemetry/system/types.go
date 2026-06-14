package system

import (
	"context"
	"errors"
	"time"
)

type Clock func() time.Time

type Source string

const (
	SourceProcess     Source = "ollama-process"
	SourceConnections Source = "ollama-loopback-connections"
	SourceHost        Source = "host-metrics"
)

type Provider string

const (
	DefaultProcessProvider    Provider = "windows-process-provider"
	DefaultConnectionProvider Provider = "windows-connection-provider"
	DefaultHostProvider       Provider = "windows-host-provider"
)

type SnapshotStatus string

const (
	StatusConfirmed   SnapshotStatus = "confirmed"
	StatusUnavailable SnapshotStatus = "unavailable"
	StatusUnsupported SnapshotStatus = "unsupported"
)

var ErrUnsupported = errors.New("unsupported provider")

type SnapshotMeta struct {
	Source     Source
	Provider   Provider
	ObservedAt time.Time
	Status     SnapshotStatus
	Healthy    bool
	Reachable  bool
	Supported  bool
	Error      string
}

type ProcessSample struct {
	PID        int32
	Name       string
	CPUPercent float64
	RSSBytes   uint64
	ReadBytes  uint64
	WriteBytes uint64
}

type ProcessSnapshot struct {
	Meta    SnapshotMeta
	Found   bool
	Process ProcessSample
}

type ConnectionSample struct {
	PID           int32
	LocalAddress  string
	LocalPort     uint32
	RemoteAddress string
	RemotePort    uint32
	State         string
}

type ConnectionsSnapshot struct {
	Meta        SnapshotMeta
	PID         int32
	Connections []ConnectionSample
}

type HostMetrics struct {
	CPUPercent       float64
	MemoryUsedBytes  uint64
	MemoryTotalBytes uint64
	SwapUsedBytes    uint64
	SwapTotalBytes   uint64
}

type HostSnapshot struct {
	Meta    SnapshotMeta
	Metrics HostMetrics
}

type ProcessProvider interface {
	LookupOllama(context.Context) (ProcessSample, bool, error)
}

type ConnectionProvider interface {
	Connections(context.Context) ([]ConnectionSample, error)
}

type HostMetricsProvider interface {
	HostMetrics(context.Context) (HostMetrics, error)
}

func resolveClock(clock Clock) Clock {
	if clock != nil {
		return clock
	}

	return time.Now
}

func metaSuccess(source Source, provider Provider, observedAt time.Time) SnapshotMeta {
	return SnapshotMeta{
		Source:     source,
		Provider:   provider,
		ObservedAt: observedAt,
		Status:     StatusConfirmed,
		Healthy:    true,
		Reachable:  true,
		Supported:  true,
	}
}

func metaFailure(source Source, provider Provider, observedAt time.Time, err error) SnapshotMeta {
	status := StatusUnavailable
	supported := true
	if errors.Is(err, ErrUnsupported) {
		status = StatusUnsupported
		supported = false
	}

	return SnapshotMeta{
		Source:     source,
		Provider:   provider,
		ObservedAt: observedAt,
		Status:     status,
		Healthy:    false,
		Reachable:  false,
		Supported:  supported,
		Error:      err.Error(),
	}
}
