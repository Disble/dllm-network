package system

import (
	"context"
	"fmt"
	"runtime"
)

type ProcessCollector struct {
	provider ProcessProvider
	clock    Clock
}

func NewProcessCollector(provider ProcessProvider, clock Clock) *ProcessCollector {
	if provider == nil {
		provider = defaultProcessProvider{}
	}

	return &ProcessCollector{
		provider: provider,
		clock:    resolveClock(clock),
	}
}

func (collector *ProcessCollector) Collect(ctx context.Context) ProcessSnapshot {
	observedAt := collector.clock()
	sample, found, err := collector.provider.LookupOllama(ctx)
	if err != nil {
		return ProcessSnapshot{
			Meta: metaFailure(SourceProcess, DefaultProcessProvider, observedAt, err),
		}
	}

	return ProcessSnapshot{
		Meta:    metaSuccess(SourceProcess, DefaultProcessProvider, observedAt),
		Found:   found,
		Process: sample,
	}
}

type defaultProcessProvider struct{}

func (defaultProcessProvider) LookupOllama(context.Context) (ProcessSample, bool, error) {
	return ProcessSample{}, false, fmt.Errorf("%s platform process lookup is not wired yet: %w", runtime.GOOS, ErrUnsupported)
}
