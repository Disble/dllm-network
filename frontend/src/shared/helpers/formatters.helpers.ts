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
 * formatBytes renders byte counts into small human-readable labels.
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

  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}
