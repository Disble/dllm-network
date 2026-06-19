import type { InferenceEvent } from '../contracts/dashboard-snapshot.types';
import { PHASE_IN_PROGRESS } from '../contracts/dashboard-snapshot.types';

/**
 * deriveDisplayTiming resolves the timing to display for one inference, given the
 * current wall-clock `nowMS`.
 *
 * Honesty contract (matches the backend's "never fabricate"):
 *  - Completed/cancelled (tokens present): use the measured durations.
 *  - In progress (tokens still null): the TOTAL is the live elapsed wall-clock
 *    (now − request start) — a real, observed measurement, so the waterfall and
 *    the total tick upward live. The load/eval PHASES stay null because the split
 *    is genuinely unknown until completion; we never invent them.
 *  - Anything else (metadata-only, unknown): all null.
 */
export function deriveDisplayTiming(
  event: InferenceEvent,
  nowMS: number,
): { loadMS: number | null; evalMS: number | null; totalMS: number | null } {
  const tokens = event.tokens;
  if (tokens != null) {
    return {
      loadMS: tokens.loadDuration / 1e6,
      evalMS: tokens.evalDuration / 1e6,
      totalMS: tokens.latencyMS,
    };
  }

  if (event.status === PHASE_IN_PROGRESS) {
    const startedAt = Date.parse(event.at);
    const totalMS = Number.isNaN(startedAt) ? null : Math.max(0, nowMS - startedAt);
    return { loadMS: null, evalMS: null, totalMS };
  }

  return { loadMS: null, evalMS: null, totalMS: null };
}
