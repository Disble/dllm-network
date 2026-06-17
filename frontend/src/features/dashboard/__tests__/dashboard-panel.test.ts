import { createElement } from 'react';
import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { DashboardPanel } from '../dashboard-panel';
import { createDashboardViewModel } from '../dashboard-view-model.helpers';
import type { DashboardSnapshot } from '../../../shared/contracts/dashboard-snapshot.types';

describe('DashboardPanel', () => {
  it('renders separate confirmed and inferred sections with passive limits', () => {
    const snapshot = createSnapshot();
    const viewModel = createDashboardViewModel(snapshot, new Date('2026-06-15T00:00:30Z'));

    render(createElement(DashboardPanel, { viewModel }));

    expect(screen.getByText('Confirmed telemetry')).toBeTruthy();
    expect(screen.getByText('Inferred activity')).toBeTruthy();
    expect(screen.getByText('Passive limits')).toBeTruthy();
    expect(screen.getByText('Exact request latency is unavailable in passive mode.')).toBeTruthy();
    expect(screen.getByText('Exact streaming chunks are unavailable in passive mode.')).toBeTruthy();
    expect(screen.getByText('high confidence')).toBeTruthy();
    expect(screen.getByText('mistral')).toBeTruthy();
  });
});

function createSnapshot(): DashboardSnapshot {
  return {
    publishedAt: '2026-06-15T00:00:00Z',
    confirmed: {
      ollama: {
        status: 'confirmed',
        reachable: true,
        version: '0.8.0',
        primaryModel: 'mistral',
        runningModels: ['mistral'],
        catalogModelCount: 2,
        observedAt: '2026-06-14T23:59:55Z',
        lastConfirmedAt: '2026-06-14T23:59:55Z',
      },
      system: {
        observedAt: '2026-06-14T23:59:57Z',
        process: {
          status: 'confirmed',
          found: true,
          pid: 4242,
          cpuPercent: 12.3,
          rssBytes: 1024,
        },
        connections: {
          status: 'confirmed',
          count: 1,
        },
        host: {
          status: 'confirmed',
          cpuPercent: 20.4,
          memoryUsedBytes: 4096,
          memoryTotalBytes: 8192,
        },
      },
    },
    inferred: {
      current: {
        kind: 'inferred-model-loaded',
        truth: 'inferred',
        model: 'mistral',
        confidence: 'high',
        observedAt: '2026-06-14T23:59:59Z',
        evidence: [{ kind: 'confirmed-running-model', detail: 'confirmed running model: mistral' }],
      },
      recent: [],
    },
    recent: {
      confirmedModels: [{ observedAt: '2026-06-14T23:58:00Z', model: 'gemma3' }],
    },
    health: {
      ollama: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-14T23:59:55Z', error: '' },
      process: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-14T23:59:57Z', error: '' },
      connections: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-14T23:59:57Z', error: '' },
      host: { status: 'confirmed', healthy: true, supported: true, observedAt: '2026-06-14T23:59:57Z', error: '' },
    },
    passive: {
      mode: 'passive-only',
      exactRequestLatencyAvailable: false,
      exactTokenCountsAvailable: false,
      exactPayloadAvailable: false,
      exactStatusAvailable: false,
      exactStreamingChunksAvailable: false,
      notes: [
        'Exact request latency is unavailable in passive mode.',
        'Exact token counts are unavailable in passive mode.',
        'Exact request and response payloads are unavailable in passive mode.',
        'Exact HTTP status results are unavailable in passive mode.',
        'Exact streaming chunks are unavailable in passive mode.',
      ],
    },
    inference: { current: { at: '', endpoint: '', method: '', model: '', promptSize: 0, streaming: false, status: 0, tokens: null }, recent: [] },
  };
}
