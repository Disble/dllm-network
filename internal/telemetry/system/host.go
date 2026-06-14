package system

import (
	"context"
	"fmt"
	"runtime"
)

type HostCollector struct {
	provider HostMetricsProvider
	clock    Clock
}

func NewHostCollector(provider HostMetricsProvider, clock Clock) *HostCollector {
	if provider == nil {
		provider = defaultHostMetricsProvider{}
	}

	return &HostCollector{
		provider: provider,
		clock:    resolveClock(clock),
	}
}

func (collector *HostCollector) Collect(ctx context.Context) HostSnapshot {
	observedAt := collector.clock()
	metrics, err := collector.provider.HostMetrics(ctx)
	if err != nil {
		return HostSnapshot{
			Meta: metaFailure(SourceHost, DefaultHostProvider, observedAt, err),
		}
	}

	return HostSnapshot{
		Meta:    metaSuccess(SourceHost, DefaultHostProvider, observedAt),
		Metrics: metrics,
	}
}

type defaultHostMetricsProvider struct{}

func (defaultHostMetricsProvider) HostMetrics(context.Context) (HostMetrics, error) {
	return HostMetrics{}, fmt.Errorf("%s platform host metrics are not wired yet: %w", runtime.GOOS, ErrUnsupported)
}
