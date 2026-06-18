/**
 * Tests for RunningModelCard (presentational) + RunningModelsContainer (container).
 * Covers enriched field rendering + honest absent-field handling.
 */
import { createElement } from 'react';
import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { EMPTY_DASHBOARD_SNAPSHOT } from '../../../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot, RunningModelView } from '../../../shared/contracts/dashboard-snapshot.types';
import type { DashboardSnapshotSource } from '../../../infrastructure/dashboard-snapshot-source';
import { RunningModelCard } from '../running-model-card';
import { RunningModelsContainer } from '../running-models-container';
import { buildRunningModelCardViewModel } from '../running-model-card-view-model.helpers';

afterEach(() => {
  cleanup();
});

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const createSourceController = (initialSnapshot: DashboardSnapshot) => {
  const listeners = new Set<Parameters<DashboardSnapshotSource['subscribe']>[0]>();
  let currentSnapshot = initialSnapshot;

  return {
    emit(snapshot: DashboardSnapshot) {
      currentSnapshot = snapshot;
      for (const listener of listeners) listener(snapshot);
    },
    source: {
      subscribe(listener: Parameters<DashboardSnapshotSource['subscribe']>[0]) {
        listeners.add(listener);
        return () => { listeners.delete(listener); };
      },
      getSnapshot() { return currentSnapshot; },
    },
  };
};

const makeRunningModel = (overrides?: Partial<RunningModelView>): RunningModelView => ({
  name: 'llama3:8b',
  size: 4_500_000_000,
  sizeVram: 4_200_000_000,
  parameterSize: '8B',
  quantizationLevel: 'Q4_0',
  contextLength: 8192,
  expiresAt: '2026-06-16T14:00:00Z',
  ...overrides,
});

const makeSnapshotWithModels = (models: readonly RunningModelView[]): DashboardSnapshot => ({
  ...EMPTY_DASHBOARD_SNAPSHOT,
  confirmed: {
    ...EMPTY_DASHBOARD_SNAPSHOT.confirmed,
    ollama: {
      ...EMPTY_DASHBOARD_SNAPSHOT.confirmed.ollama,
      runningModelDetails: models,
    },
  },
});

// ---------------------------------------------------------------------------
// buildRunningModelCardViewModel — pure mapping
// ---------------------------------------------------------------------------

describe('buildRunningModelCardViewModel', () => {
  it('maps all enriched fields to display strings', () => {
    const model = makeRunningModel();
    const vm = buildRunningModelCardViewModel(model);

    expect(vm.name).toBe('llama3:8b');
    expect(vm.parameterSize).toBe('8B');
    expect(vm.quantizationLevel).toBe('Q4_0');
    expect(vm.contextLengthLabel).toMatch(/8192/);
    expect(vm.sizeLabel).not.toBe('—');
    expect(vm.sizeVramLabel).not.toBe('—');
    // expiresAt: should produce a human-readable relative or absolute label
    expect(vm.expiresAtLabel).not.toBe('');
  });

  it('renders "—" for absent parameterSize', () => {
    const model = makeRunningModel({ parameterSize: '' });
    const vm = buildRunningModelCardViewModel(model);
    expect(vm.parameterSize).toBe('—');
  });

  it('renders "—" for absent quantizationLevel', () => {
    const model = makeRunningModel({ quantizationLevel: '' });
    const vm = buildRunningModelCardViewModel(model);
    expect(vm.quantizationLevel).toBe('—');
  });

  it('renders "0 B" for zero size', () => {
    const model = makeRunningModel({ size: 0 });
    const vm = buildRunningModelCardViewModel(model);
    expect(vm.sizeLabel).toBe('0 B');
  });

  it('renders "0 B" for zero sizeVram', () => {
    const model = makeRunningModel({ sizeVram: 0 });
    const vm = buildRunningModelCardViewModel(model);
    expect(vm.sizeVramLabel).toBe('0 B');
  });

  it('renders "—" for absent expiresAt', () => {
    const model = makeRunningModel({ expiresAt: '' });
    const vm = buildRunningModelCardViewModel(model);
    expect(vm.expiresAtLabel).toBe('—');
  });

  it('renders contextLength as a plain number string', () => {
    const model = makeRunningModel({ contextLength: 4096 });
    const vm = buildRunningModelCardViewModel(model);
    expect(vm.contextLengthLabel).toContain('4096');
  });
});

// ---------------------------------------------------------------------------
// RunningModelCard — presentational
// ---------------------------------------------------------------------------

describe('RunningModelCard', () => {
  it('renders all enriched model fields', () => {
    const model = makeRunningModel();
    const vm = buildRunningModelCardViewModel(model);

    render(createElement(RunningModelCard, { viewModel: vm }));

    expect(screen.getByText('llama3:8b')).toBeTruthy();
    expect(screen.getByText('8B')).toBeTruthy();
    expect(screen.getByText('Q4_0')).toBeTruthy();
    expect(screen.getByText(/8192/)).toBeTruthy();
    // size and sizeVram should appear in human-readable form
    expect(screen.queryAllByText(/GB|MB|KB|B/).length).toBeGreaterThan(0);
  });

  it('renders "—" for absent parameterSize and quantizationLevel', () => {
    const model = makeRunningModel({ parameterSize: '', quantizationLevel: '' });
    const vm = buildRunningModelCardViewModel(model);

    render(createElement(RunningModelCard, { viewModel: vm }));

    // At least 2 dashes for the two absent fields
    const dashes = screen.getAllByText('—');
    expect(dashes.length).toBeGreaterThanOrEqual(2);
  });

  it('renders "—" for absent expiresAt', () => {
    const model = makeRunningModel({ expiresAt: '' });
    const vm = buildRunningModelCardViewModel(model);

    render(createElement(RunningModelCard, { viewModel: vm }));

    expect(screen.getByText('—')).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// RunningModelsContainer — container
// ---------------------------------------------------------------------------

describe('RunningModelsContainer', () => {
  it('renders a card for each running model in the snapshot', () => {
    const models = [makeRunningModel({ name: 'llama3:8b' }), makeRunningModel({ name: 'mistral:7b' })];
    const controller = createSourceController(makeSnapshotWithModels(models));

    render(createElement(RunningModelsContainer, { source: controller.source }));

    expect(screen.getByText('2 active')).toBeTruthy();
    expect(screen.getByText('llama3:8b')).toBeTruthy();
    expect(screen.getByText('mistral:7b')).toBeTruthy();
  });

  it('renders empty-state message when no models are running', () => {
    const controller = createSourceController(EMPTY_DASHBOARD_SNAPSHOT);

    render(createElement(RunningModelsContainer, { source: controller.source }));

    expect(screen.getByText('Loaded models')).toBeTruthy();
    expect(screen.getByText('0 active')).toBeTruthy();
    expect(screen.getByText(/no running models/i)).toBeTruthy();
  });

  it('updates the model list when snapshot changes', () => {
    const controller = createSourceController(EMPTY_DASHBOARD_SNAPSHOT);

    render(createElement(RunningModelsContainer, { source: controller.source }));

    act(() => {
      controller.emit(makeSnapshotWithModels([makeRunningModel({ name: 'llama3:8b' })]));
    });

    expect(screen.getByText('llama3:8b')).toBeTruthy();
  });
});
