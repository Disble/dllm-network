import { createElement } from 'react';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { EMPTY_DASHBOARD_SNAPSHOT } from '../../../shared/contracts/dashboard-snapshot.constants';
import { DashboardPanel } from '../dashboard-panel';
import { createDashboardViewModel } from '../dashboard-view-model.helpers';

afterEach(() => {
  cleanup();
});

const viewModel = createDashboardViewModel(
  { ...EMPTY_DASHBOARD_SNAPSHOT, publishedAt: '2026-06-15T00:00:00Z' },
  new Date('2026-06-15T00:00:30Z'),
);

describe('DashboardPanel', () => {
  it('renders the compact summary header and three tiles', () => {
    render(createElement(DashboardPanel, { viewModel }));

    expect(screen.getByText('Passive-only telemetry')).toBeTruthy();
    expect(screen.getByText('dllm-network')).toBeTruthy();
    expect(screen.getByText('Collection mode')).toBeTruthy();
    expect(screen.getByText('Snapshot time')).toBeTruthy();
    expect(screen.getByText('Status')).toBeTruthy();
    expect(screen.getByText('Passive-only')).toBeTruthy();
    expect(screen.getByText('Fresh passive snapshot')).toBeTruthy();
  });

  it('drops the verbose warnings, notes and inferred clutter', () => {
    render(createElement(DashboardPanel, { viewModel }));

    expect(screen.queryByText('Passive limits')).toBeNull();
    expect(screen.queryByText('Confirmed telemetry')).toBeNull();
    expect(screen.queryByText(/unavailable in passive mode/i)).toBeNull();
    expect(screen.queryByText(/confidence/i)).toBeNull();
  });
});
