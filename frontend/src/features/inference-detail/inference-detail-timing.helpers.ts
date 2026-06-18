import { UNAVAILABLE_LABEL } from './inference-detail.constants';

/**
 * Formats a millisecond duration as a rounded value with an "ms" suffix,
 * or the configured unavailable sentinel when the value is null.
 */
export function formatTimingMilliseconds(value: number | null): string {
  return value === null ? UNAVAILABLE_LABEL : `${Math.round(value)}ms`;
}
