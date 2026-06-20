import { describe, expect, it } from 'vitest';

import {
  formatClockDateTime,
  formatClockTime,
  formatTimestamp,
} from '../formatters.helpers';

// These assertions run under the suite's pinned zone (TZ=America/Bogota, UTC-5;
// see vitest.setup.ts). They lock the contract that timestamps render in the
// HOST'S local time with an explicit offset — never UTC mislabelled as "Z".
describe('local-time timestamp formatters', () => {
  it('formatClockTime renders local HH:MM:SS without an offset (dense column)', () => {
    expect(formatClockTime('2026-06-18T14:27:02Z')).toBe('09:27:02');
  });

  it('formatClockDateTime renders local date-time with an explicit ±HH:MM offset', () => {
    expect(formatClockDateTime('2026-06-18T14:27:02Z')).toBe('2026-06-18 09:27:02 -05:00');
  });

  it('formatTimestamp renders local date-time with an explicit ±HH:MM offset', () => {
    expect(formatTimestamp('2026-06-18T14:27:02Z')).toBe('2026-06-18 09:27:02 -05:00');
  });

  it('rolls the calendar date back when the local offset crosses midnight', () => {
    // 02:00Z in UTC-5 is the previous day at 21:00 — the DATE must shift too.
    expect(formatClockDateTime('2026-06-15T02:00:00Z')).toBe('2026-06-14 21:00:00 -05:00');
    expect(formatClockTime('2026-06-15T02:00:00Z')).toBe('21:00:00');
  });

  it('accepts a source carrying an explicit offset, not just "Z"', () => {
    // Same absolute instant as 14:27:02Z, expressed with a +02:00 offset.
    expect(formatClockTime('2026-06-18T16:27:02+02:00')).toBe('09:27:02');
  });

  it('returns honest fallbacks for empty or unparseable input', () => {
    expect(formatClockTime('')).toBe('—');
    expect(formatClockDateTime('')).toBe('—');
    expect(formatTimestamp('')).toBe('Unavailable');
    expect(formatClockTime('not-a-date')).toBe('—');
    expect(formatTimestamp('not-a-date')).toBe('Unavailable');
  });
});
