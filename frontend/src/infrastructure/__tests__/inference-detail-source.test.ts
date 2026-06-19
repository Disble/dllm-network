import { afterEach, describe, expect, it, vi } from 'vitest';

import type { InferenceEvent } from '../../shared/contracts/dashboard-snapshot.types';
import { createInferenceDetailSource } from '../inference-detail-source';

type WindowWithGo = typeof window & {
  go?: { app?: { App?: { InferenceDetail?: (id: string) => Promise<InferenceEvent> } } };
};

const record = (id: string): InferenceEvent => ({
  at: '2026-06-19T00:00:00Z',
  endpoint: '/api/generate',
  method: 'POST',
  model: 'llama3',
  promptSize: 10,
  streaming: false,
  status: 1,
  tokens: null,
  id,
  requestBody: 'the prompt',
  responseBody: 'the full response',
});

const setBinding = (fn?: (id: string) => Promise<InferenceEvent>) => {
  (window as WindowWithGo).go = fn ? { app: { App: { InferenceDetail: fn } } } : undefined;
};

afterEach(() => {
  delete (window as WindowWithGo).go;
});

describe('createInferenceDetailSource', () => {
  it('returns null when the Wails binding is absent (plain browser)', async () => {
    const src = createInferenceDetailSource();
    expect(await src.fetchDetail('inf-1')).toBeNull();
  });

  it('returns null for an empty id without calling the binding', async () => {
    const spy = vi.fn(async () => record('x'));
    setBinding(spy);
    const src = createInferenceDetailSource();
    expect(await src.fetchDetail('')).toBeNull();
    expect(spy).not.toHaveBeenCalled();
  });

  it('returns the full record from the binding', async () => {
    setBinding(async (id) => record(id));
    const src = createInferenceDetailSource();
    const got = await src.fetchDetail('inf-9');
    expect(got?.id).toBe('inf-9');
    expect(got?.responseBody).toBe('the full response');
  });

  it('treats the backend zero value (empty id) as not found', async () => {
    setBinding(async () => record(''));
    const src = createInferenceDetailSource();
    expect(await src.fetchDetail('missing')).toBeNull();
  });

  it('returns null when the binding rejects', async () => {
    setBinding(async () => {
      throw new Error('boom');
    });
    const src = createInferenceDetailSource();
    expect(await src.fetchDetail('inf-1')).toBeNull();
  });
});
