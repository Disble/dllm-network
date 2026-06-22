import { afterEach, describe, expect, it, vi } from 'vitest';

import type { InferenceEvent } from '../../shared/contracts/dashboard-snapshot.types';
import { createInferenceDetailSource } from '../inference-detail-source';

// eslint-disable-next-line no-unused-vars
type DetailBinding = (id: string) => Promise<string>;
type WindowWithGo = typeof window & {
  go?: { app?: { App?: { InferenceDetail?: DetailBinding } } };
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

const setBinding = (fn?: DetailBinding) => {
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
    const spy = vi.fn<DetailBinding>(async (id) => JSON.stringify(record(id)));
    setBinding(spy);
    const src = createInferenceDetailSource();
    expect(await src.fetchDetail('')).toBeNull();
    expect(spy).not.toHaveBeenCalled();
  });

  it('parses the JSON record returned by the binding', async () => {
    setBinding(async (id) => JSON.stringify(record(id)));
    const src = createInferenceDetailSource();
    const got = await src.fetchDetail('inf-9');
    expect(got?.id).toBe('inf-9');
    expect(got?.responseBody).toBe('the full response');
  });

  it('treats an empty string (backend not-found) as null', async () => {
    setBinding(async () => '');
    const src = createInferenceDetailSource();
    expect(await src.fetchDetail('missing')).toBeNull();
  });

  it('returns null when the JSON is malformed', async () => {
    setBinding(async () => 'not json');
    const src = createInferenceDetailSource();
    expect(await src.fetchDetail('inf-1')).toBeNull();
  });

  it('returns null when the binding rejects', async () => {
    setBinding(async () => {
      throw new Error('boom');
    });
    const src = createInferenceDetailSource();
    expect(await src.fetchDetail('inf-1')).toBeNull();
  });
});
