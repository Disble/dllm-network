/**
 * formatTimestamp renders a stable timestamp or fallback label for passive telemetry surfaces.
 */
export function formatTimestamp(value: string): string {
  if (value === '') {
    return 'Unavailable';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return 'Unavailable';
  }

  return date.toISOString().replace('.000Z', 'Z');
}

/**
 * formatClockTime renders just the wall-clock HH:MM:SS (UTC) from an RFC3339
 * timestamp for dense table columns. Returns "—" when empty or unparseable.
 */
export function formatClockTime(value: string): string {
  if (value === '') {
    return '—';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }

  return date.toISOString().slice(11, 19);
}

/**
 * formatBytes renders byte counts into small human-readable labels.
 * Handles B, KB, MB, and GB ranges.
 */
export function formatBytes(value: number): string {
  if (value <= 0) {
    return '0 B';
  }

  if (value < 1024) {
    return `${value} B`;
  }

  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }

  if (value < 1024 * 1024 * 1024) {
    return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  }

  return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

/**
 * formatExpiresAt renders an ISO-8601 expires_at timestamp as a relative time label.
 * Returns "—" when the value is empty or unparseable.
 * Returns the formatted time if more than 0 seconds remain; "expired" if in the past.
 */
export function formatExpiresAt(value: string, now?: Date): string {
  if (value === '') {
    return '—';
  }

  const expiresDate = new Date(value);
  if (Number.isNaN(expiresDate.getTime())) {
    return '—';
  }

  const referenceTime = now ?? new Date();
  const diffMs = expiresDate.getTime() - referenceTime.getTime();

  if (diffMs <= 0) {
    return 'expired';
  }

  const diffSec = Math.floor(diffMs / 1000);
  const minutes = Math.floor(diffSec / 60);
  const seconds = diffSec % 60;

  if (minutes === 0) {
    return `in ${seconds}s`;
  }

  return `in ${minutes}m ${seconds}s`;
}
