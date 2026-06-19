import type { InferenceEvent } from '../shared/contracts/dashboard-snapshot.types';

/**
 * InferenceDetailSource fetches the full inference record (request/response
 * bodies + headers) for a selected row on demand. The high-frequency dashboard
 * snapshot ships only metadata for the recent list; bodies are loaded lazily
 * here when a row is selected — mirroring how Chrome DevTools loads a request's
 * response body only when you click it.
 */
export interface InferenceDetailSource {
  /** Fetch the full record for id, or null when unavailable (no Wails runtime, persistence disabled, in-progress/unknown id). */
  // eslint-disable-next-line no-unused-vars -- Function-type param documents the fetch contract.
  readonly fetchDetail: (id: string) => Promise<InferenceEvent | null>;
}

// InferenceDetailBinding is the Wails-generated App.InferenceDetail method shape,
// injected on window.go inside the desktop webview. It returns the record as a
// JSON STRING (the Go binding marshals it; the domain type's time.Time fields
// are not mappable by Wails' TS model generator), or "" when there is no record.
// eslint-disable-next-line no-unused-vars -- Function-type param documents the Wails binding contract.
type InferenceDetailBinding = (id: string) => Promise<string>;

/**
 * createInferenceDetailSource returns the runtime-backed detail source. It calls
 * the Wails App.InferenceDetail binding when running inside the desktop webview,
 * and degrades to returning null in a plain browser (vite dev) or when the
 * backend reports no record (zero value, empty id).
 */
export function createInferenceDetailSource(): InferenceDetailSource {
  return {
    async fetchDetail(id) {
      if (id === '') {
        return null;
      }

      const binding = (
        window as typeof window & {
          go?: { app?: { App?: { InferenceDetail?: InferenceDetailBinding } } };
        }
      ).go?.app?.App?.InferenceDetail;

      if (binding === undefined) {
        return null;
      }

      try {
        // The backend returns "" for an unknown id or when persistence is
        // unavailable; treat that as "no detail".
        const json = await binding(id);
        if (!json) {
          return null;
        }
        const record = JSON.parse(json) as InferenceEvent;
        return record.id ? record : null;
      } catch {
        return null;
      }
    },
  };
}

/**
 * inferenceDetailSource is the shared runtime-backed detail source for feature hooks.
 */
export const inferenceDetailSource = createInferenceDetailSource();
