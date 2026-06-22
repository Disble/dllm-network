/**
 * Tests the on-demand body fetch in useInferenceDetail: the recent list ships
 * metadata only, so selecting a terminal row fetches its full record (bodies)
 * from the injected detail source; in-progress rows (no persisted record) fall
 * back to the live store event.
 */
import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import type { InferenceDetailSource } from '../../../infrastructure/inference-detail-source';
import { EMPTY_DASHBOARD_SNAPSHOT } from '../../../shared/contracts/dashboard-snapshot.constants';
import type { DashboardSnapshot, InferenceEvent } from '../../../shared/contracts/dashboard-snapshot.types';
import { resetInferenceStore, useInferenceStore } from '../../../shared/store/inference-store';
import { useInferenceDetail } from '../use-inference-detail';

beforeEach(() => {
  resetInferenceStore();
});

afterEach(() => {
  resetInferenceStore();
});

describe('useInferenceDetail on-demand body fetch', () => {
  const strippedTerminal: InferenceEvent = {
    at: '2026-06-19T04:20:00Z',
    endpoint: '/v1/chat/completions',
    method: 'POST',
    model: 'gemma4:12b',
    promptSize: 1234,
    streaming: true,
    status: 1,
    tokens: null,
    id: 'inf-1',
    // bodies intentionally absent — the snapshot recent list is metadata-only
  };

  const seed = (event: InferenceEvent, asRecent: boolean) => {
    const snapshot: DashboardSnapshot = {
      ...EMPTY_DASHBOARD_SNAPSHOT,
      inference: {
        current: asRecent ? EMPTY_DASHBOARD_SNAPSHOT.inference.current : event,
        recent: asRecent ? [event] : [],
      },
    };
    act(() => {
      useInferenceStore.getState().ingest(snapshot);
    });
  };
  it('fetches the full record for the selected terminal row', async () => {
    seed(strippedTerminal, true);

    const full: InferenceEvent = { ...strippedTerminal, responseBody: 'the full response', requestBody: 'the prompt' };
    const detailSource: InferenceDetailSource = {
      fetchDetail: async (id) => (id === 'inf-1' ? full : null),
    };

    const { result } = renderHook(() => useInferenceDetail(undefined, detailSource));
    act(() => {
      useInferenceStore.getState().select('inf-1');
    });

    await waitFor(() => {
      expect(result.current.event?.responseBody).toBe('the full response');
    });
  });

  it('falls back to the live store event when no record is fetched (in-progress)', async () => {
    const inProgress: InferenceEvent = {
      ...strippedTerminal,
      id: 'inf-2',
      status: 0,
      requestBody: 'live prompt',
    };
    seed(inProgress, false);

    const detailSource: InferenceDetailSource = { fetchDetail: async () => null };

    const { result } = renderHook(() => useInferenceDetail(undefined, detailSource));
    act(() => {
      useInferenceStore.getState().select('inf-2');
    });

    await waitFor(() => {
      expect(result.current.event?.requestBody).toBe('live prompt');
    });
  });
});
